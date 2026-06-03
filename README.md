# answerlayer-go

A small, dependency-free Go client for the [AnswerLayer](https://www.answerlayer.io) **Inquiry API** — ask questions in natural language and get back governed SQL, streamed answers, and citations into the underlying rows.

This is the starting point for the official AnswerLayer Go SDK. Today it covers the inquiry endpoints end to end; the surface will grow over time.

## Status

This package is ready for early Inquiry API integrations: it supports session management, one-shot asks, streaming asks, references, SQL events, usage metadata, custom transports, and multi-tenant subject headers.

The SDK is currently `v0.x`. Until `v1.0.0`, expect the package to stay focused on Inquiry while the broader AnswerLayer API surface and compatibility guarantees mature.

```go
client := answerlayer.NewClient(os.Getenv("ANSWERLAYER_API_KEY"))

session, _ := client.CreateSession(ctx, answerlayer.CreateSessionRequest{
    ConnectionID: connectionID,
})

stream, _ := client.AskStream(ctx, session.ID, "What was revenue last quarter?")
defer stream.Close()

for stream.Next() {
    ev := stream.Event()
    if ev.Type == answerlayer.EventTextDelta {
        fmt.Print(ev.Content) // the answer, token by token
    }
}
```

## Install

```bash
go get github.com/answerlayer/answerlayer-go
```

Requires Go 1.21+. No third-party dependencies — standard library only.

## Quick start: the interactive REPL

The [`examples/repl`](examples/repl) program is a full natural-language chat loop in ~200 lines. It opens one session and lets you ask follow-up questions that build on the conversation, showing the agent's reasoning, the tools it calls, and the SQL it runs as they stream in.

```bash
export ANSWERLAYER_API_KEY=sk-...
export ANSWERLAYER_CONNECTION_ID=<your-connection-uuid>

go run ./examples/repl
```

```
● session 5f3c… ready
● ask a question, or /quit to exit

you ▸ which 5 customers spent the most last month?
⚙ run_sql §1.1(a)
  sql SELECT customer_name, SUM(amount) AS spend FROM orders WHERE …
ans ◂ Your top five customers last month were Acme ($48,200), …
  references
    [1] §1.1(a) row 1 → Acme
  1,204 in / 312 out tokens · 2 credits

you ▸ how does that compare to the month before?
```

## Authentication

Create an API key in the AnswerLayer dashboard under **Settings → API Keys** (the full key is shown only once). Pass it to `NewClient`:

```go
client := answerlayer.NewClient("sk-...")
```

When you embed AnswerLayer into a **multi-tenant product**, also identify the end user so requests are attributed and isolated per tenant. This sets the `X-Subject-Org-ID` and `X-Subject-User-ID` headers:

```go
client := answerlayer.NewClient("sk-...",
    answerlayer.WithSubject("acme-widgets", "user-42"),
)
```

Other options:

```go
answerlayer.WithBaseURL("https://your-install.example.com/api/v1") // self-hosted / staging
answerlayer.WithHTTPClient(myHTTPClient)                            // custom transport
```

## API

### Sessions

```go
session, err := client.CreateSession(ctx, answerlayer.CreateSessionRequest{
    ConnectionID: connectionID, // required
    Model:        "",           // optional model override
})

session, err := client.GetSession(ctx, sessionID)   // hydrated, with turns
list,    err := client.ListSessions(ctx, &answerlayer.ListSessionsOptions{Limit: 20})
err = client.DeleteSession(ctx, sessionID)
```

Sessions are **stateful**: keep using the same `session.ID` and the agent has the full conversation as context for follow-ups.

### Asking questions

**Streaming** (recommended — surfaces progress live):

```go
stream, err := client.AskStream(ctx, sessionID, "your question")
if err != nil { /* ... */ }
defer stream.Close()

for stream.Next() {
    ev := stream.Event()
    switch ev.Type {
    case answerlayer.EventThinking:    // ev.Content — agent reasoning
    case answerlayer.EventToolStart:   // ev.ToolName, ev.ToolInput
    case answerlayer.EventSQLExecuted: // ev.SQL
    case answerlayer.EventTextDelta:   // ev.Content — answer chunk
    case answerlayer.EventComplete:    // ev.FinalResponse, ev.References, usage
    case answerlayer.EventError:       // ev.Message
    }
}
if err := stream.Err(); err != nil { /* ... */ }
```

**One-shot** (blocks until done, returns just the answer):

```go
res, err := client.Ask(ctx, sessionID, "your question")
fmt.Println(res.FinalResponse)
```

### Streaming event types

Each `Event` is a flat union — switch on `ev.Type`, then read the fields documented for that type.

| Type | Key fields | Meaning |
|------|-----------|---------|
| `EventTurnCreated` | `TurnID` | Turn record created |
| `EventStart` | `Model`, `Timestamp` | Agent work started |
| `EventStage` | `Content` | Coarse progress note |
| `EventThinking` | `Content`, `Section` | Agent reasoning |
| `EventToolStart` | `ToolName`, `ToolInput`, `Section` | Tool invoked |
| `EventToolEnd` | `ToolSuccess`, `ToolResult`, `ToolDurationMs` | Tool finished |
| `EventSQLExecuted` | `SQL` | A SQL statement ran |
| `EventResultSummary` | `Summary` | Short summary of a result |
| `EventTextDelta` | `Content` | Incremental answer text |
| `EventComplete` | `FinalResponse`, `References`, `SQLQueries`, `InputTokens`, `OutputTokens`, `CreditCost` | Final answer + usage |
| `EventError` | `Message` | Turn failed |

`References` link a marker in the answer (e.g. `[1]`) back to a specific row in the queried data, so you can show provenance for every claim.

### Errors

Non-2xx responses return an `*answerlayer.APIError`:

```go
var apiErr *answerlayer.APIError
if errors.As(err, &apiErr) {
    fmt.Println(apiErr.StatusCode, apiErr.Detail)
}
```

## Contributing

Issues and PRs welcome. This client is intended to grow into the full AnswerLayer Go SDK, so contributions that extend coverage to other endpoints are especially appreciated.

## License

[Apache 2.0](LICENSE)
