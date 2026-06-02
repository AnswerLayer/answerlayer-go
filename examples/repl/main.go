// Command repl is an interactive natural-language client for AnswerLayer.
//
// It opens one inquiry session and then loops: you type a question, it streams
// the agent's progress (reasoning, tools, SQL) and the answer, and you can ask
// follow-ups that build on the same conversation.
//
// Usage:
//
//	export ANSWERLAYER_API_KEY=sk-...
//	export ANSWERLAYER_CONNECTION_ID=<connection-uuid>
//	go run ./examples/repl
//
// Optional environment variables:
//
//	ANSWERLAYER_BASE_URL   override the API root (self-hosted / staging)
//	ANSWERLAYER_ORG_ID     X-Subject-Org-ID  (multi-tenant attribution)
//	ANSWERLAYER_USER_ID    X-Subject-User-ID (multi-tenant attribution)
//	NO_COLOR               disable ANSI colors when set
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	answerlayer "github.com/answerlayer/answerlayer-go"
)

func main() {
	apiKey := os.Getenv("ANSWERLAYER_API_KEY")
	connectionID := os.Getenv("ANSWERLAYER_CONNECTION_ID")
	if apiKey == "" || connectionID == "" {
		fmt.Fprintln(os.Stderr, "set ANSWERLAYER_API_KEY and ANSWERLAYER_CONNECTION_ID")
		os.Exit(1)
	}

	opts := []answerlayer.Option{}
	if base := os.Getenv("ANSWERLAYER_BASE_URL"); base != "" {
		opts = append(opts, answerlayer.WithBaseURL(base))
	}
	if org, user := os.Getenv("ANSWERLAYER_ORG_ID"), os.Getenv("ANSWERLAYER_USER_ID"); org != "" || user != "" {
		opts = append(opts, answerlayer.WithSubject(org, user))
	}
	client := answerlayer.NewClient(apiKey, opts...)

	// Cancel the in-flight turn on Ctrl-C without killing the whole program.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	session, err := client.CreateSession(ctx, answerlayer.CreateSessionRequest{
		ConnectionID: connectionID,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", red("could not start session:"), err)
		os.Exit(1)
	}

	fmt.Printf("%s session %s ready\n", dim("●"), session.ID)
	fmt.Printf("%s ask a question, or /quit to exit\n\n", dim("●"))

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		fmt.Print(bold("you ▸ "))
		if !in.Scan() {
			break // EOF (Ctrl-D)
		}
		q := strings.TrimSpace(in.Text())
		switch q {
		case "":
			continue
		case "/quit", "/exit", "/q":
			fmt.Println(dim("bye"))
			return
		}
		if err := ask(ctx, client, session.ID, q); err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Printf("\n%s\n\n", dim("(cancelled)"))
				continue
			}
			fmt.Fprintf(os.Stderr, "\n%s %v\n\n", red("error:"), err)
		}
	}
	fmt.Println(dim("bye"))
}

// ask streams one turn and renders it to the terminal.
func ask(ctx context.Context, client *answerlayer.Client, sessionID, question string) error {
	stream, err := client.AskStream(ctx, sessionID, question)
	if err != nil {
		return err
	}
	defer stream.Close()

	answering := false
	for stream.Next() {
		ev := stream.Event()
		switch ev.Type {
		case answerlayer.EventStage:
			// Transient status on a single, overwritten line.
			fmt.Printf("\r\033[K%s %s", dim("…"), dim(ev.Content))

		case answerlayer.EventThinking:
			fmt.Printf("\r\033[K%s %s\n", dim(ev.Section), dim(truncate(ev.Content, 160)))

		case answerlayer.EventToolStart:
			fmt.Printf("\r\033[K%s %s\n", cyan("⚙ "+ev.ToolName), dim(ev.Section))

		case answerlayer.EventSQLExecuted:
			fmt.Printf("  %s %s\n", dim("sql"), dim(oneLine(ev.SQL)))

		case answerlayer.EventTextDelta:
			if !answering {
				fmt.Printf("\r\033[K%s ", bold("ans ◂"))
				answering = true
			}
			fmt.Print(ev.Content)

		case answerlayer.EventComplete:
			// If the server didn't stream deltas, print the assembled answer.
			if !answering && ev.FinalResponse != "" {
				fmt.Printf("\r\033[K%s %s", bold("ans ◂"), ev.FinalResponse)
			}
			fmt.Println()
			printReferences(ev.References)
			fmt.Printf("%s\n\n", dim(fmt.Sprintf(
				"  %d in / %d out tokens · %d credits",
				ev.InputTokens, ev.OutputTokens, ev.CreditCost)))

		case answerlayer.EventError:
			return errors.New(ev.Message)
		}
	}
	return stream.Err()
}

func printReferences(refs []answerlayer.Reference) {
	if len(refs) == 0 {
		return
	}
	fmt.Println(dim("  references"))
	for _, r := range refs {
		line := fmt.Sprintf("    [%s] %s row %d", r.Display, r.Section, r.Row)
		if r.Value != nil {
			line += fmt.Sprintf(" → %v", r.Value)
		}
		fmt.Println(dim(line))
	}
}

func truncate(s string, n int) string {
	s = oneLine(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// --- minimal ANSI helpers (honor NO_COLOR) ---

var noColor = os.Getenv("NO_COLOR") != ""

func color(code, s string) string {
	if noColor {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}

func dim(s string) string  { return color("2", s) }
func bold(s string) string { return color("1", s) }
func cyan(s string) string { return color("36", s) }
func red(s string) string  { return color("31", s) }
