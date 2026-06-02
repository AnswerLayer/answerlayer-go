package answerlayer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError is returned for any non-2xx HTTP response from the AnswerLayer API.
type APIError struct {
	// StatusCode is the HTTP status code (e.g. 401, 404, 422).
	StatusCode int
	// Detail is the FastAPI "detail" field when present. For 422 validation
	// errors it holds the raw JSON array of validation problems.
	Detail string
	// Body is the raw response body, useful when Detail could not be parsed.
	Body string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("answerlayer: http %d: %s", e.StatusCode, e.Detail)
	}
	return fmt.Sprintf("answerlayer: http %d: %s", e.StatusCode, e.Body)
}

func parseAPIError(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	e := &APIError{StatusCode: resp.StatusCode, Body: string(b)}

	var p struct {
		Detail json.RawMessage `json:"detail"`
	}
	if json.Unmarshal(b, &p) == nil && len(p.Detail) > 0 {
		// detail is a string for most errors, or an array for 422 validation.
		var s string
		if json.Unmarshal(p.Detail, &s) == nil {
			e.Detail = s
		} else {
			e.Detail = string(p.Detail)
		}
	}
	return e
}
