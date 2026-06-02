package answerlayer

import "time"

// Session is a multi-turn inquiry conversation bound to a single database
// connection. CreateSession returns a Session with only ID and the values you
// supplied populated; GetSession returns the fully hydrated record.
type Session struct {
	ID            string    `json:"id"`
	ConnectionID  string    `json:"connection_id"`
	Status        string    `json:"status"` // active, paused, completed
	Model         string    `json:"model"`
	ThreadSummary string    `json:"thread_summary"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Turns         []Turn    `json:"turns"`
}

// SessionSummary is the lightweight session shape returned by ListSessions.
type SessionSummary struct {
	ID            string    `json:"id"`
	ConnectionID  string    `json:"connection_id"`
	Status        string    `json:"status"`
	Model         string    `json:"model"`
	ThreadSummary string    `json:"thread_summary"`
	TurnCount     int       `json:"turn_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Turn is a single question/answer exchange within a session.
type Turn struct {
	ID              string      `json:"id"`
	SessionID       string      `json:"session_id"`
	UserInput       string      `json:"user_input"`
	FinalResponse   string      `json:"final_response"`
	SQLQueries      []string    `json:"sql_queries"`
	ResultSummaries []string    `json:"result_summaries"`
	References      []Reference `json:"references"`
	InputTokens     int         `json:"input_tokens"`
	OutputTokens    int         `json:"output_tokens"`
	CreatedAt       time.Time   `json:"created_at"`
}

// Reference is a citation linking a marker in the answer text (for example
// "[1]") back to a specific row in the data the agent queried.
type Reference struct {
	// Display is the marker shown in the answer, e.g. "1".
	Display string `json:"display"`
	// Section is the internal provenance path, e.g. "§1.2(a)".
	Section string `json:"section"`
	// Row is the 1-indexed row number within the referenced result.
	Row int `json:"row"`
	// Value is the cited cell value (may be a string, number, or null).
	Value any `json:"value"`
}
