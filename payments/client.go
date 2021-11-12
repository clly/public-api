package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp-demoapp/public-api/models"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// HTTPClient is a concrete implmentation of an HTTP client which can communicate with the payments service
type HTTPClient struct {
	client  *http.Client
	baseURL string
}

func NewHTTP(baseURL string) *HTTPClient {

	c := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	return &HTTPClient{c, baseURL}
}

// MakePayment calls the payments api
func (h *HTTPClient) MakePayment(ctx context.Context, details *models.PaymentDetails) (*models.PaymentResponse, error) {
	pr := &PaymentRequest{}
	pr.FromModel(details)

	req, err := http.NewRequestWithContext(ctx, "POST", h.baseURL, pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	// decode the body
	prResp := PaymentResponse{}
	err = json.NewDecoder(resp.Body).Decode(&prResp)
	if err != nil {
		return nil, err
	}

	prModel := &models.PaymentResponse{}
	prResp.ToModel(prModel)

	return prModel, nil
}

// PaymentRequest defines the JSON request for the Payments API
type PaymentRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Number string `json:"number"`
	Expiry string `json:"expiry"`
	CVC    string `json:"cvc"`

	readIndex int64
	buffer    []byte
}

// FromModel converts a graphql model into a payment request
func (pr *PaymentRequest) FromModel(m *models.PaymentDetails) {
	pr.CVC = fmt.Sprintf("%d", m.Cv2)
	pr.Expiry = m.Expiry
	pr.Name = m.Name
	pr.Number = m.Number
	pr.Type = m.Type
}

func (pr *PaymentRequest) Read(p []byte) (n int, err error) {
	// if this is first read marshal the struct
	if pr.readIndex == 0 {
		pr.buffer, err = json.Marshal(pr)
		if err != nil {
			return 0, err
		}
	}

	// we have read all there is to read, reset
	if pr.readIndex >= int64(len(pr.buffer)) {
		err = io.EOF
		pr.readIndex = 0
		return
	}

	n = copy(p, pr.buffer[pr.readIndex:])
	pr.readIndex += int64(n)
	return
}

type PaymentResponse struct {
	ID             string `json:"id"`
	Message        string `json:"message"`
	CardPlaintext  string `json:"card_plaintext"`
	CardCiphertext string `json:"card_ciphertext"`
}

// ToModel converts a go Struct into a payment response model
func (pr *PaymentResponse) ToModel(m *models.PaymentResponse) {
	m.ID = pr.ID
	m.Message = pr.Message
	m.CardCiphertext = pr.CardCiphertext
	m.CardPlaintext = pr.CardPlaintext
}
