package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/handler"
	"github.com/hashicorp-demoapp/product-api-go/client"
	"github.com/hashicorp-demoapp/public-api/auth"
	"github.com/hashicorp-demoapp/public-api/models"
	"github.com/hashicorp-demoapp/public-api/payments"
	"github.com/hashicorp-demoapp/public-api/resolver"
	"github.com/hashicorp-demoapp/public-api/server"
	"github.com/hashicorp/go-hclog"
	"github.com/nicholasjackson/env"
	"github.com/opentracing/opentracing-go"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel"
	otlpgrpc "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	otbridge "go.opentelemetry.io/otel/bridge/opentracing"

	// "github.com/hashicorp-demoapp/public-api/service"
	"github.com/gorilla/mux"
	"github.com/keratin/authn-go/authn"
)

var logger hclog.Logger

var bindAddress = env.String("BIND_ADDRESS", false, ":8080", "Bind address for the server")
var metricsAddress = env.String("METRICS_ADDRESS", false, ":9102", "Metrics address for the server")
var productAddress = env.String("PRODUCT_API_URI", false, "http://localhost:9090", "Address for the product api")
var paymentAddress = env.String("PAYMENT_API_URI", false, "http://localhost:18000", "Address for the payment api")

const SERVICE_NAME = "public-api"

func main() {

	ctx, closer, err := InitTracer()
	if err != nil {
		log.Fatal(err)
	}
	defer closer()

	logger = hclog.New(&hclog.LoggerOptions{
		Name:  "public-api",
		Level: hclog.Debug,
	})

	ctx, span := otel.Tracer("public-api").Start(ctx, "init")
	err = env.Parse()
	if err != nil {
		log.Fatal(err)
	}

	// Config.
	// config := service.NewConfig()

	// Authentication.
	authn, err := authn.NewClient(authn.Config{
		// The AUTHN_URL of your Keratin AuthN server. This will be used to verify tokens created by
		// AuthN, and will also be used for API calls unless PrivateBaseURL is also set.
		Issuer: "http://localhost",

		// The domain of your application (no protocol). This domain should be listed in the APP_DOMAINS
		// of your Keratin AuthN server.
		Audience: "localhost",

		// Credentials for AuthN's private endpoints. These will be used to execute admin actions using
		// the Client provided by this library.
		//
		// TIP: make them extra secure in production!
		Username: "hello",
		Password: "world",

		// RECOMMENDED: Send private API calls to AuthN using private network routing. This can be
		// necessary if your environment has a firewall to limit public endpoints.
		PrivateBaseURL: "http://localhost",
	})

	if err != nil {
		log.Fatal(err)
	}

	// Server.
	r := mux.NewRouter()
	r.Use(otelmux.Middleware(SERVICE_NAME))
	r.Use(auth.Middleware(authn))

	// create the client to the products-api
	productsClient := client.NewHTTP(*productAddress)

	// create the client for the payments-api
	paymentClient := payments.NewHTTP(*paymentAddress)

	// Graphql.
	c := server.Config{
		Resolvers: resolver.NewResolver(productsClient, paymentClient, logger),
	}

	// Check if the user is authenticated.
	c.Directives.IsAuthenticated = func(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {
		isAuthenticated := auth.IsAuthenticated(ctx)
		if !isAuthenticated {
			return nil, fmt.Errorf("Access denied")
		}

		return next(ctx)
	}

	// Check if the user has a role.
	c.Directives.HasRole = func(ctx context.Context, obj interface{}, next graphql.Resolver, role models.Role) (interface{}, error) {
		logger.Debug("Auth has role", "role", role)
		return next(ctx)
	}

	// Handlers.
	r.Handle("/", handler.Playground("Playground", "/api"))
	r.Handle("/api", handler.GraphQL(server.NewExecutableSchema(c)))

	logger.Info("Starting server", "bind", *bindAddress, "metrics", *metricsAddress)

	span.End()
	err = http.ListenAndServe(*bindAddress, r)
	if err != nil {
		logger.Error("Unable to start server", "error", err)
		os.Exit(1)
	}
}

func InitTracer() (context.Context, func(), error) {
	ctx := context.Background()

	//otel.SetErrorHandler()

	var exporter sdktrace.SpanExporter // allows overwrite in --test mode
	var err error

	exporter, err = otlpgrpc.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to configure OTLP/GRPC exporter: %s", err)
	}

	// set the service name that will show up in tracing UIs
	resAttrs := resource.WithAttributes(semconv.ServiceNameKey.String(SERVICE_NAME))
	res, err := resource.New(ctx, resAttrs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OpenTelemetry service name resource: %s", err)
	}

	// SSP sends all completed spans to the exporter immediately and that is
	// exactly what we want/need in this app
	// https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/trace/simple_span_processor.go
	ssp := sdktrace.NewBatchSpanProcessor(exporter)

	// ParentBased/AlwaysSample Sampler is the default and that's fine for this
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(ssp),
	)

	// inject the tracer into the otel globals (and this starts the background stuff, I think)
	otel.SetTracerProvider(tracerProvider)

	// set up the W3C trace context as the global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	bridgeTracer, wrappedProvider := otbridge.NewTracerPair(otel.GetTracerProvider().Tracer(""))
	//closer, err := hckit.InitGlobalTracer("public-api")
	otel.SetTracerProvider(wrappedProvider)

	bridgeTracer.SetWarningHandler(func(msg string) {
		hclog.Default().Warn(msg)
	})
	opentracing.SetGlobalTracer(bridgeTracer)

	// callers need to defer this to make sure all the data gets flushed out
	return ctx, func() {
		//closer.Close()
		err = tracerProvider.Shutdown(ctx)
		if err != nil {
			hclog.Default().Error("shutdown of OpenTelemetry tracerProvider failed: %s", err)
		}

		err = exporter.Shutdown(ctx)
		if err != nil {
			hclog.Default().Error("shutdown of OpenTelemetry OTLP exporter failed: %s", err)
		}
	}, nil
}
