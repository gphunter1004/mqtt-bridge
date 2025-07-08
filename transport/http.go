package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// HTTPTransport implements the MessageTransport interface for HTTP communication.
type HTTPTransport struct {
	client  *http.Client
	headers map[string]string
	logger  *slog.Logger
}

// NewHTTPTransport creates a new instance of HTTPTransport.
func NewHTTPTransport(timeout time.Duration, userAgent string, logger *slog.Logger) *HTTPTransport {
	if userAgent == "" {
		userAgent = "MQTT-Bridge-Client/1.0"
	}
	return &HTTPTransport{
		client: &http.Client{
			Timeout: timeout,
		},
		headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   userAgent,
		},
		logger: logger.With("transport_type", "http"),
	}
}

// Send dispatches a payload to a URL via an HTTP POST request.
func (ht *HTTPTransport) Send(ctx context.Context, url string, payload []byte) error {
	logger := ht.logger.With("url", url)
	logger.Debug("Sending POST request", "payload_size", len(payload))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		logger.Error("Failed to create HTTP request", slog.Any("error", err))
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	for key, value := range ht.headers {
		req.Header.Set(key, value)
	}

	resp, err := ht.client.Do(req)
	if err != nil {
		logger.Error("HTTP request failed", slog.Any("error", err))
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
		logger.Error("Received non-success HTTP status", "status_code", resp.StatusCode, "response_body", string(body))
		return err
	}

	logger.Info("Request successful", "status_code", resp.StatusCode)
	return nil
}

// GetTransportType returns the transport's type.
func (ht *HTTPTransport) GetTransportType() TransportType {
	return TransportTypeHTTP
}

// Close cleans up idle connections.
func (ht *HTTPTransport) Close() error {
	ht.client.CloseIdleConnections()
	ht.logger.Info("Idle connections closed.")
	return nil
}

// SetHeader allows setting custom headers for all subsequent requests.
func (ht *HTTPTransport) SetHeader(key, value string) {
	ht.headers[key] = value
	ht.logger.Debug("Custom header set", "key", key)
}
