module github.com/hashicorp-demoapp/public-api

go 1.13

require (
	github.com/99designs/gqlgen v0.12.2
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp-demoapp/product-api-go v0.0.12
	github.com/hashicorp/go-hclog v0.14.1
	github.com/keratin/authn-go v1.1.0
	github.com/magiconair/properties v1.8.3 // indirect
	github.com/nicholasjackson/env v0.6.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/vektah/gqlparser/v2 v2.0.1
	go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux v0.26.1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.26.1
	go.opentelemetry.io/otel v1.1.0
	go.opentelemetry.io/otel/bridge/opentracing v1.1.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.1.0
	go.opentelemetry.io/otel/sdk v1.1.0
)

replace github.com/hashicorp-demoapp/product-api-go => ../product-api-go
