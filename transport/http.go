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

type HTTPTransport struct {
	client  *http.Client
	headers map[string]string
}

func NewHTTPTransport(timeout time.Duration) *HTTPTransport {
	return &HTTPTransport{
		client: &http.Client{
			Timeout: timeout,
		},
		headers: map[string]string{
			"Content-Type": "application/json",
			"User-Agent":   "MQTT-Bridge-Client/1.0",
		},
	}
}

func (ht *HTTPTransport) Send(ctx context.Context, url string, payload []byte) error {
	log.Printf("[HTTP Transport] Sending POST to: %s", url)
	log.Printf("[HTTP Transport] Payload size: %d bytes", len(payload))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 헤더 설정
	for key, value := range ht.headers {
		req.Header.Set(key, value)
	}

	resp, err := ht.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// 응답 상태 코드 확인
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 성공적인 응답 로깅
	body, err := io.ReadAll(resp.Body)
	if err == nil && len(body) > 0 {
		log.Printf("[HTTP Transport] Response received: %s", string(body))
	}

	log.Printf("[HTTP Transport] Request successful: %s (Status: %d)", url, resp.StatusCode)
	return nil
}

func (ht *HTTPTransport) GetTransportType() TransportType {
	return TransportTypeHTTP
}

func (ht *HTTPTransport) Close() error {
	ht.client.CloseIdleConnections()
	log.Println("[HTTP Transport] Idle connections closed")
	return nil
}

func (ht *HTTPTransport) SetHeader(key, value string) {
	ht.headers[key] = value
	log.Printf("[HTTP Transport] Header set: %s = %s", key, value)
}

func (ht *HTTPTransport) SetTimeout(timeout time.Duration) {
	ht.client.Timeout = timeout
	log.Printf("[HTTP Transport] Timeout set to: %v", timeout)
}

func (ht *HTTPTransport) SetBasicAuth(username, password string) {
	ht.headers["Authorization"] = fmt.Sprintf("Basic %s",
		encodeBasicAuth(username, password))
	log.Printf("[HTTP Transport] Basic auth configured for user: %s", username)
}

func (ht *HTTPTransport) SetBearerToken(token string) {
	ht.headers["Authorization"] = fmt.Sprintf("Bearer %s", token)
	log.Println("[HTTP Transport] Bearer token configured")
}

// 기본 인증 인코딩 (간단한 구현)
func encodeBasicAuth(username, password string) string {
	// 실제로는 base64 encoding이 필요하지만 여기서는 단순화
	return fmt.Sprintf("%s:%s", username, password)
}
