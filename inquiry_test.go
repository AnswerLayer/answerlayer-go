package answerlayer

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestAskEscapesSessionIDAndDecodesResult(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.URL.EscapedPath(); got != "/inquiry/sessions/sess%2Fwith%20space/sync" {
			t.Fatalf("escaped path = %s, want escaped session path", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["user_input"] != "What changed?" {
			t.Fatalf("user_input = %q, want question", body["user_input"])
		}

		resp := response(http.StatusOK, `{"turn_id":"turn-1","final_response":"Revenue increased.","sql_queries":["SELECT 1"]}`)
		resp.Header.Set("Content-Type", "application/json")
		return resp, nil
	})}

	client := NewClient("sk-test", WithBaseURL("https://example.test"), WithHTTPClient(httpClient))
	res, err := client.Ask(context.Background(), "sess/with space", "What changed?")
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	if res.TurnID != "turn-1" || res.FinalResponse != "Revenue increased." || len(res.SQLQueries) != 1 || res.SQLQueries[0] != "SELECT 1" {
		t.Fatalf("result = %+v, want decoded ask result", res)
	}
}

func TestAskStreamParsesServerSentEvents(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.URL.EscapedPath(); got != "/inquiry/sessions/sess%2Fwith%20space" {
			t.Fatalf("escaped path = %s, want escaped session path", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Fatalf("Accept = %q, want text/event-stream", got)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if body["user_input"] != "Top customers?" {
			t.Fatalf("user_input = %q, want question", body["user_input"])
		}

		resp := response(http.StatusOK, "event: message\n"+
			`data: {"type":"text_delta","content":"Top "}`+"\n\n"+
			": keepalive\n\n"+
			`data: {"type":"complete","turn_id":"turn-1","final_response":"Top customers","references":[{"display":"1","section":"§1.1(a)","row":1,"value":"Acme"}],"sql_queries":["SELECT customer_name FROM customers"],"input_tokens":12,"output_tokens":8,"credit_cost":1}`+"\n")
		resp.Header.Set("Content-Type", "text/event-stream")
		return resp, nil
	})}

	client := NewClient("sk-test", WithBaseURL("https://example.test"), WithHTTPClient(httpClient))
	stream, err := client.AskStream(context.Background(), "sess/with space", "Top customers?")
	if err != nil {
		t.Fatalf("AskStream returned error: %v", err)
	}
	defer stream.Close()

	if !stream.Next() {
		t.Fatalf("first Next = false, err = %v", stream.Err())
	}
	first := stream.Event()
	if first.Type != EventTextDelta || first.Content != "Top " {
		t.Fatalf("first event = %+v, want text delta", first)
	}

	if !stream.Next() {
		t.Fatalf("second Next = false, err = %v", stream.Err())
	}
	second := stream.Event()
	if second.Type != EventComplete || second.TurnID != "turn-1" || second.FinalResponse != "Top customers" {
		t.Fatalf("second event = %+v, want complete", second)
	}
	if len(second.References) != 1 || second.References[0].Display != "1" || second.References[0].Value != "Acme" {
		t.Fatalf("references = %+v, want decoded reference", second.References)
	}
	if len(second.SQLQueries) != 1 || second.SQLQueries[0] != "SELECT customer_name FROM customers" {
		t.Fatalf("SQLQueries = %+v, want decoded query", second.SQLQueries)
	}
	if second.InputTokens != 12 || second.OutputTokens != 8 || second.CreditCost != 1 {
		t.Fatalf("usage = %d/%d/%d, want 12/8/1", second.InputTokens, second.OutputTokens, second.CreditCost)
	}

	if stream.Next() {
		t.Fatalf("third Next = true, want end of stream")
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream.Err() = %v, want nil", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

func TestAskStreamInvalidEventSetsErr(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		resp := response(http.StatusOK, "data: {not json}\n\n")
		resp.Header.Set("Content-Type", "text/event-stream")
		return resp, nil
	})}

	client := NewClient("sk-test", WithBaseURL("https://example.test"), WithHTTPClient(httpClient))
	stream, err := client.AskStream(context.Background(), "sess-1", "Question?")
	if err != nil {
		t.Fatalf("AskStream returned error: %v", err)
	}
	defer stream.Close()

	if stream.Next() {
		t.Fatal("Next = true, want false for invalid JSON")
	}
	if stream.Err() == nil {
		t.Fatal("Err = nil, want JSON parse error")
	}
}
