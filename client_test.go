package answerlayer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCreateSessionSendsAuthSubjectHeadersAndJSON(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/inquiry/sessions" {
			t.Fatalf("path = %s, want /api/v1/inquiry/sessions", r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "sk-test" {
			t.Fatalf("X-API-Key = %q, want sk-test", got)
		}
		if got := r.Header.Get("X-Subject-Org-ID"); got != "org-1" {
			t.Fatalf("X-Subject-Org-ID = %q, want org-1", got)
		}
		if got := r.Header.Get("X-Subject-User-ID"); got != "user-1" {
			t.Fatalf("X-Subject-User-ID = %q, want user-1", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var body CreateSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body.ConnectionID != "conn-1" || body.Model != "claude-test" {
			t.Fatalf("body = %+v, want connection and model", body)
		}

		resp := response(http.StatusOK, `{"session_id":"sess-1"}`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}

	client := NewClient("sk-test",
		WithBaseURL("https://example.test/api/v1/"),
		WithSubject("org-1", "user-1"),
		WithHTTPClient(httpClient),
	)
	session, err := client.CreateSession(context.Background(), CreateSessionRequest{
		ConnectionID: "conn-1",
		Model:        "claude-test",
	})
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if session.ID != "sess-1" || session.ConnectionID != "conn-1" || session.Model != "claude-test" || session.Status != "active" {
		t.Fatalf("session = %+v, want populated session", session)
	}
}

func TestListSessionsEncodesOptionsAndDecodesResponse(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/inquiry/sessions" {
			t.Fatalf("path = %s, want /inquiry/sessions", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("connection_id") != "conn-1" || q.Get("status") != "active" || q.Get("limit") != "20" || q.Get("offset") != "40" {
			t.Fatalf("query = %s, want connection_id/status/limit/offset", r.URL.RawQuery)
		}
		resp := response(http.StatusOK, `[{"id":"sess-1","connection_id":"conn-1","status":"active","model":"claude","thread_summary":"summary","turn_count":2}]`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}

	client := NewClient("sk-test", WithBaseURL("https://example.test"), WithHTTPClient(httpClient))
	sessions, err := client.ListSessions(context.Background(), &ListSessionsOptions{
		ConnectionID: "conn-1",
		Status:       "active",
		Limit:        20,
		Offset:       40,
	})
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != "sess-1" || sessions[0].TurnCount != 2 {
		t.Fatalf("sessions = %+v, want decoded session summary", sessions)
	}
}

func TestAPIErrorParsesStringAndValidationDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantDetail string
	}{
		{
			name:       "string detail",
			body:       `{"detail":"bad key"}`,
			wantDetail: "bad key",
		},
		{
			name:       "structured detail",
			body:       `{"detail":[{"loc":["body","connection_id"],"msg":"required"}]}`,
			wantDetail: `[{"loc":["body","connection_id"],"msg":"required"}]`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				resp := response(http.StatusUnauthorized, tt.body)
				resp.Header.Set("Content-Type", "application/json")
				return resp, nil
			})}

			client := NewClient("sk-test", WithBaseURL("https://example.test"), WithHTTPClient(httpClient))
			_, err := client.GetSession(context.Background(), "sess-1")
			if err == nil {
				t.Fatal("GetSession returned nil error, want APIError")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %T, want *APIError", err)
			}
			if apiErr.StatusCode != http.StatusUnauthorized {
				t.Fatalf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
			}
			if !strings.Contains(apiErr.Detail, tt.wantDetail) {
				t.Fatalf("Detail = %q, want to contain %q", apiErr.Detail, tt.wantDetail)
			}
		})
	}
}
