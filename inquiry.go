package answerlayer

import (
	"bufio"
	"context"
	"encoding/json"
	"net/url"
	"strings"
)

// AskResult is the response from the non-streaming Ask call.
type AskResult struct {
	TurnID        string   `json:"turn_id"`
	FinalResponse string   `json:"final_response"`
	SQLQueries    []string `json:"sql_queries"`
}

// Ask submits a question and blocks until the agent has finished, returning the
// final answer. Use AskStream if you want to surface progress as it happens.
func (c *Client) Ask(ctx context.Context, sessionID, question string) (*AskResult, error) {
	var out AskResult
	body := map[string]string{"user_input": question}
	path := "/inquiry/sessions/" + url.PathEscape(sessionID) + "/sync"
	if err := c.doJSON(ctx, "POST", path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EventType identifies the kind of streaming event. Switch on Event.Type to
// decide which fields of the Event are populated.
type EventType string

const (
	// EventTurnCreated is emitted first, carrying the new turn's ID.
	EventTurnCreated EventType = "turn_created"
	// EventStart marks the start of agent work (Model, Timestamp).
	EventStart EventType = "start"
	// EventStage is a coarse human-readable progress note (Content).
	EventStage EventType = "stage"
	// EventThinking is the agent's reasoning text (Content, Section).
	EventThinking EventType = "thinking"
	// EventToolStart is emitted when the agent invokes a tool
	// (ToolID, ToolName, ToolInput, Section).
	EventToolStart EventType = "tool_start"
	// EventToolEnd is emitted when a tool returns
	// (ToolID, ToolName, ToolSuccess, ToolResult, ToolDurationMs, Section).
	EventToolEnd EventType = "tool_end"
	// EventSQLExecuted carries a SQL statement the agent ran (SQL).
	EventSQLExecuted EventType = "sql_executed"
	// EventResultSummary carries a short summary of a query result (Summary).
	EventResultSummary EventType = "result_summary"
	// EventTextDelta is an incremental chunk of the final answer (Content).
	EventTextDelta EventType = "text_delta"
	// EventComplete is the final event with the assembled answer and usage.
	EventComplete EventType = "complete"
	// EventError signals that the turn failed (Message).
	EventError EventType = "error"
)

// Event is a single server-sent event from AskStream. It is a flat union: only
// the fields relevant to Type are set. Inspect Type, then read the documented
// fields for that type.
type Event struct {
	Type EventType `json:"type"`

	// EventStage, EventThinking, EventTextDelta
	Content string `json:"content"`
	// EventThinking, EventToolStart, EventToolEnd
	Section string `json:"section"`
	// EventTurnCreated, EventComplete
	TurnID string `json:"turn_id"`
	// EventStart
	Model string `json:"model"`
	// EventStart, EventToolStart, EventToolEnd (Unix seconds)
	Timestamp float64 `json:"timestamp"`

	// EventToolStart, EventToolEnd
	ToolID         string          `json:"tool_id"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolSuccess    bool            `json:"tool_success"`
	ToolResult     string          `json:"tool_result"`
	ToolDurationMs int             `json:"tool_duration_ms"`

	// EventSQLExecuted
	SQL string `json:"sql"`
	// EventResultSummary
	Summary string `json:"summary"`
	// EventError
	Message string `json:"message"`

	// EventComplete
	FinalResponse      string      `json:"final_response"`
	References         []Reference `json:"references"`
	SQLQueries         []string    `json:"sql_queries"`
	ToolCallCount      int         `json:"tool_call_count"`
	ThinkingBlockCount int         `json:"thinking_block_count"`
	InputTokens        int         `json:"input_tokens"`
	OutputTokens       int         `json:"output_tokens"`
	CreditCost         int         `json:"credit_cost"`
}

// Stream is an iterator over the server-sent events of a single inquiry turn.
// Drive it with Next/Event and always Close it when done:
//
//	for stream.Next() {
//	    ev := stream.Event()
//	    // ...
//	}
//	if err := stream.Err(); err != nil { ... }
type Stream struct {
	closer  interface{ Close() error }
	scanner *bufio.Scanner
	cur     Event
	err     error
	done    bool
}

// AskStream submits a question and returns a Stream of progress events ending
// in an EventComplete (or EventError). The provided context cancels the
// underlying HTTP request; cancelling it stops the stream.
func (c *Client) AskStream(ctx context.Context, sessionID, question string) (*Stream, error) {
	body := map[string]string{"user_input": question}
	path := "/inquiry/sessions/" + url.PathEscape(sessionID)
	req, err := c.newRequest(ctx, "POST", path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseAPIError(resp)
	}

	sc := bufio.NewScanner(resp.Body)
	// Allow large single-event payloads (the complete event can be sizeable).
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return &Stream{closer: resp.Body, scanner: sc}, nil
}

// Next advances to the next event, returning false at end of stream or on
// error. Check Err after the loop to distinguish the two.
func (s *Stream) Next() bool {
	if s.done {
		return false
	}

	var data []string
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if line == "" {
			// Blank line terminates an SSE event.
			if len(data) == 0 {
				continue // ignore leading/duplicate blank lines
			}
			return s.emit(data)
		}
		// We only care about data: fields; event:/comment lines are ignored
		// because the API discriminates on the "type" key inside the payload.
		if v, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(v, " "))
		}
	}

	if err := s.scanner.Err(); err != nil {
		s.err = err
	}
	s.done = true
	// Flush a trailing event that wasn't terminated by a blank line.
	if len(data) > 0 {
		return s.emit(data)
	}
	return false
}

func (s *Stream) emit(data []string) bool {
	var ev Event
	if err := json.Unmarshal([]byte(strings.Join(data, "\n")), &ev); err != nil {
		s.err = err
		s.done = true
		return false
	}
	s.cur = ev
	return true
}

// Event returns the event produced by the most recent successful Next.
func (s *Stream) Event() Event { return s.cur }

// Err returns the first non-EOF error encountered while streaming, if any.
func (s *Stream) Err() error { return s.err }

// Close releases the underlying HTTP connection. It is safe to call more than
// once.
func (s *Stream) Close() error {
	if s.closer == nil {
		return nil
	}
	c := s.closer
	s.closer = nil
	return c.Close()
}
