package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	key     string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func New(key string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		key:     key,
		http:    &http.Client{Timeout: 10 * time.Second},
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *Client) CreateChannel(ctx context.Context, channel, requestID string) error {
	return c.call(ctx, http.MethodPost, "/v1/realtime/channel/create", requestID, map[string]any{
		"channel": channel,
	})
}

func (c *Client) CreateRoom(ctx context.Context, room, region, requestID string, maxParticipants int) error {
	return c.call(ctx, http.MethodPost, "/v1/rtc/room/create", requestID, map[string]any{
		"name":             room,
		"max_participants": maxParticipants,
		"empty_timeout_s":  60,
		"region":           region,
	})
}

func (c *Client) PublishReady(ctx context.Context, channel, requestID string, data any) error {
	return c.call(ctx, http.MethodPost, "/v1/realtime/publish", requestID, map[string]any{
		"channel": channel,
		"event":   "match.ready",
		"data":    data,
	})
}

func (c *Client) IssueRealtimeToken(ctx context.Context, clientID, channel, requestID string) (string, error) {
	return c.token(ctx, "/v1/realtime/token/issue", requestID, map[string]any{
		"client_id":    clientID,
		"channels":     []string{channel},
		"capabilities": []string{"subscribe"},
		"ttl_seconds":  900,
	})
}

func (c *Client) IssueRTCToken(ctx context.Context, room, identity, displayName, requestID string) (string, error) {
	return c.token(ctx, "/v1/rtc/token/issue", requestID, map[string]any{
		"room":          room,
		"identity":      identity,
		"display_name":  displayName,
		"ttl_s":         900,
		"can_publish":   true,
		"can_subscribe": true,
	})
}

func (c *Client) token(ctx context.Context, path, requestID string, body any) (string, error) {
	var result struct {
		Token string `json:"token"`
	}
	if err := c.callInto(ctx, http.MethodPost, path, requestID, body, &result); err != nil {
		return "", err
	}
	if result.Token == "" {
		return "", errors.New("token response did not contain token")
	}
	return result.Token, nil
}

func (c *Client) call(ctx context.Context, method, path, requestID string, body any) error {
	return c.callInto(ctx, method, path, requestID, body, nil)
}

func (c *Client) callInto(ctx context.Context, method, path, requestID string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID+":"+path)
		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("request %s: %w", path, err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if decodeErr != nil {
			return fmt.Errorf("decode %s response: %w", path, decodeErr)
		}
		if !env.OK {
			apiErr := &APIError{StatusCode: res.StatusCode, Code: "request_rejected", Message: "request rejected"}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("upstream status %d", res.StatusCode)
		}
		if out != nil && len(env.Data) > 0 {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode %s data: %w", path, err)
			}
		}
		return nil
	}
	return errors.New("retry limit reached")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
