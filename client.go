package answerlayer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// DefaultBaseURL is the public AnswerLayer API root used when no override is
// supplied via WithBaseURL.
const DefaultBaseURL = "https://app.answerlayer.io/api/v1"

// Client is a handle to the AnswerLayer Inquiry API. It is safe for concurrent
// use by multiple goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	orgID      string
	userID     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (e.g. a self-hosted or staging install).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithSubject sets the end-user identity headers (X-Subject-Org-ID and
// X-Subject-User-ID) sent on every request. These are required when embedding
// AnswerLayer into a multi-tenant product so that each request is attributed to
// the right tenant and user. Either value may be empty to omit it.
func WithSubject(orgID, userID string) Option {
	return func(c *Client) { c.orgID = orgID; c.userID = userID }
}

// WithHTTPClient supplies a custom *http.Client. The default client has no
// timeout because inquiry turns stream for as long as the agent is working;
// control per-call deadlines with the context instead.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// NewClient returns a Client authenticated with the given API key.
func NewClient(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{}, // no timeout: streams can be long-lived
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	if c.orgID != "" {
		req.Header.Set("X-Subject-Org-ID", c.orgID)
	}
	if c.userID != "" {
		req.Header.Set("X-Subject-User-ID", c.userID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// doJSON performs a request whose response is a single JSON document. If out is
// nil the body is drained and discarded (e.g. for 204 responses).
func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return parseAPIError(resp)
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
