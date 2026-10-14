package infrai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCreateChannelRetriesRateLimit(t *testing.T) {
	attempts := 0
	client := New("test-key")
	client.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if req.Method != http.MethodPost || req.URL.Path != "/v1/realtime/channel/create" {
			t.Fatalf("request = %s %s", req.Method, req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer test-key" || req.Header.Get("Idempotency-Key") == "" {
			t.Fatal("missing authorization or idempotency header")
		}
		status, body := http.StatusOK, `{"ok":true,"data":{},"error":null,"metadata":{}}`
		headers := make(http.Header)
		if attempts == 1 {
			status, body = http.StatusTooManyRequests, `{"ok":false,"data":null,"error":{"code":"rate_limited","message":"retry later"},"metadata":{}}`
			headers.Set("Retry-After", "0")
		}
		return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	client.sleep = func(context.Context, time.Duration) error { return nil }

	if err := client.CreateChannel(context.Background(), "duo-eu-west-9decf9e8", "match-42"); err != nil {
		t.Fatalf("CreateChannel() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}
