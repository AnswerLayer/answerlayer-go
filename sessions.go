package answerlayer

import (
	"context"
	"net/url"
	"strconv"
)

// CreateSessionRequest is the payload for CreateSession.
type CreateSessionRequest struct {
	// ConnectionID is the UUID of the database connection to query. Required.
	ConnectionID string `json:"connection_id"`
	// Model optionally overrides the default Claude model for the session.
	Model string `json:"model,omitempty"`
}

// CreateSession opens a new inquiry session against a database connection.
//
// The API returns only the new session ID, so the returned Session has ID,
// ConnectionID, and Model populated from your request and Status set to
// "active". Call GetSession to retrieve the fully hydrated record.
func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (*Session, error) {
	var resp struct {
		SessionID string `json:"session_id"`
	}
	if err := c.doJSON(ctx, "POST", "/inquiry/sessions", req, &resp); err != nil {
		return nil, err
	}
	return &Session{
		ID:           resp.SessionID,
		ConnectionID: req.ConnectionID,
		Model:        req.Model,
		Status:       "active",
	}, nil
}

// GetSession returns a session and all of its turns.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	var s Session
	if err := c.doJSON(ctx, "GET", "/inquiry/sessions/"+url.PathEscape(sessionID), nil, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSessionsOptions filters and paginates ListSessions. The zero value lists
// the most recent sessions for the authenticated subject.
type ListSessionsOptions struct {
	ConnectionID string // filter by connection UUID
	Status       string // filter by status: active, paused, completed
	Limit        int    // 1-100; defaults to 50 server-side
	Offset       int    // pagination offset
}

// ListSessions returns sessions for the authenticated subject, newest first.
func (c *Client) ListSessions(ctx context.Context, opts *ListSessionsOptions) ([]SessionSummary, error) {
	path := "/inquiry/sessions"
	if opts != nil {
		q := url.Values{}
		if opts.ConnectionID != "" {
			q.Set("connection_id", opts.ConnectionID)
		}
		if opts.Status != "" {
			q.Set("status", opts.Status)
		}
		if opts.Limit > 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Offset > 0 {
			q.Set("offset", strconv.Itoa(opts.Offset))
		}
		if len(q) > 0 {
			path += "?" + q.Encode()
		}
	}
	var out []SessionSummary
	if err := c.doJSON(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteSession deletes a session and all of its turns.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	return c.doJSON(ctx, "DELETE", "/inquiry/sessions/"+url.PathEscape(sessionID), nil, nil)
}
