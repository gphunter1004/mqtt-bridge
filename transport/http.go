package transport

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// HTTPTransport implements the MessageTransport interface for HTTP communication.
type HTTPTransport struct {
	client  *http.Client
	headers map[string]string
}

// NewHTTPTransport creates a new instance of HTTPTransport.
// Timeout and userAgent are now passed in as arguments, typically from a config file.
func NewHTTPTransport(timeout time.Duration, userAgent string) *HTTPTransport {
	if userAgent == "" {
		userAgent = "MQTT-Bridge-Client/1.0" // Provide a default user agent if empty
	}
	return &HTTPTransport{
		client: &http.Client{
			Timeout: timeout,
		},
		headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   userAgent,
		},
	}
}

// Send dispatches a payload to a URL via an HTTP POST request.
func (ht *HTTPTransport) Send(ctx context.Context, url string, payload []byte) error {
	log.Printf("[HTTP Transport] Sending POST to: %s", url)
	log.Printf("[HTTP Transport] Payload size: %d bytes", len(payload))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set custom headers
	for key, value := range ht.headers {
		req.Header.Set(key, value)
	}

	resp, err := ht.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[HTTP Transport] Request successful: %s (Status: %d)", url, resp.StatusCode)
	return nil
}

// GetTransportType returns the transport's type.
func (ht *HTTPTransport) GetTransportType() TransportType {
	return TransportTypeHTTP
}

// Close cleans up idle connections.
func (ht *HTTPTransport) Close() error {
	ht.client.CloseIdleConnections()
	log.Println("[HTTP Transport] Idle connections closed.")
	return nil
}

// SetHeader allows setting custom headers for all subsequent requests.
func (ht *HTTPTransport) SetHeader(key, value string) {
	ht.headers[key] = value
	log.Printf("[HTTP Transport] Header set: %s = %s", key, value)
}
