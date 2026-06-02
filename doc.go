// Package answerlayer is a Go client for the AnswerLayer Inquiry API.
//
// AnswerLayer turns natural-language questions into governed SQL against your
// connected databases and streams back the answer along with the agent's
// reasoning, the SQL it ran, and citations into the underlying rows.
//
// The typical flow is:
//
//	client := answerlayer.NewClient(os.Getenv("ANSWERLAYER_API_KEY"))
//
//	session, err := client.CreateSession(ctx, answerlayer.CreateSessionRequest{
//	    ConnectionID: connectionID,
//	})
//	if err != nil { ... }
//
//	stream, err := client.AskStream(ctx, session.ID, "What was revenue last quarter?")
//	if err != nil { ... }
//	defer stream.Close()
//
//	for stream.Next() {
//	    ev := stream.Event()
//	    switch ev.Type {
//	    case answerlayer.EventTextDelta:
//	        fmt.Print(ev.Content)
//	    case answerlayer.EventComplete:
//	        fmt.Println()
//	    }
//	}
//	if err := stream.Err(); err != nil { ... }
//
// Sessions are stateful, so follow-up questions on the same session ID build on
// the prior conversation. Use [Client.Ask] for a simpler one-shot, non-streaming
// call.
//
// # Authentication
//
// Every request is authenticated with an API key sent in the X-API-Key header.
// In multi-tenant deployments you should also identify the end user with
// [WithSubject], which sets the X-Subject-Org-ID and X-Subject-User-ID headers
// for per-tenant data isolation and audit logging.
package answerlayer
