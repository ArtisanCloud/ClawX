package integration

import (
	"context"
	"sync"
	"testing"

	"clawx/internal/application/command"
	chatiface "clawx/internal/interfaces/chat"
)

func TestMultiSessionWindowRoutingWindowIsolationParallelContinue(t *testing.T) {
	stack := newTestRuntime(t)
	ctx := context.Background()

	firstA := mustCreateInWindow(t, ctx, stack, "window-a", "hello-a-1")
	firstB := mustCreateInWindow(t, ctx, stack, "window-b", "hello-b-1")
	if firstA == firstB {
		t.Fatalf("window A/B should create isolated sessions, got same id %q", firstA)
	}

	type result struct {
		sessionID string
		err       error
	}

	runParallelContinue := func(windowID, input string) <-chan result {
		ch := make(chan result, 1)
		go func() {
			defer close(ch)
			sessionID, err := continueInWindow(ctx, stack, windowID, input)
			ch <- result{sessionID: sessionID, err: err}
		}()
		return ch
	}

	chA := runParallelContinue("window-a", "hello-a-2")
	chB := runParallelContinue("window-b", "hello-b-2")

	var wg sync.WaitGroup
	wg.Add(2)

	var secondA result
	var secondB result
	go func() {
		defer wg.Done()
		secondA = <-chA
	}()
	go func() {
		defer wg.Done()
		secondB = <-chB
	}()
	wg.Wait()

	if secondA.err != nil {
		t.Fatalf("parallel continue for window-a failed: %v", secondA.err)
	}
	if secondB.err != nil {
		t.Fatalf("parallel continue for window-b failed: %v", secondB.err)
	}
	if secondA.sessionID != firstA {
		t.Fatalf("window-a should continue original session: got=%q want=%q", secondA.sessionID, firstA)
	}
	if secondB.sessionID != firstB {
		t.Fatalf("window-b should continue original session: got=%q want=%q", secondB.sessionID, firstB)
	}
	if secondA.sessionID == secondB.sessionID {
		t.Fatalf("parallel continue should stay isolated by window, got same id %q", secondA.sessionID)
	}
}

func mustContinueInWindow(t *testing.T, ctx context.Context, stack testRuntime, windowID, input string) string {
	t.Helper()

	sessionID, err := continueInWindow(ctx, stack, windowID, input)
	if err != nil {
		t.Fatalf("continue in window %q: %v", windowID, err)
	}
	return sessionID
}

func mustCreateInWindow(t *testing.T, ctx context.Context, stack testRuntime, windowID, input string) string {
	t.Helper()

	sessionID, err := createInWindow(ctx, stack, windowID, input)
	if err != nil {
		t.Fatalf("create in window %q: %v", windowID, err)
	}
	return sessionID
}

func createInWindow(ctx context.Context, stack testRuntime, windowID, input string) (string, error) {
	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "user-ms-1",
		Text:            input,
		WindowID:        windowID,
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		return "", err
	}

	decision, err := stack.router.Route(ctx, message)
	if err != nil {
		return "", err
	}
	flowResult, err := stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeNew,
		ConversationID: decision.ConversationID,
		WindowID:       decision.WindowID,
		Input:          decision.Message.Text,
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		return "", err
	}
	return flowResult.Session.ID, nil
}

func continueInWindow(ctx context.Context, stack testRuntime, windowID, input string) (string, error) {
	message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          "user-ms-1",
		Text:            input,
		WindowID:        windowID,
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	if err != nil {
		return "", err
	}

	decision, err := stack.router.Route(ctx, message)
	if err != nil {
		return "", err
	}
	flowResult, err := stack.router.HandleSessionFlow(ctx, command.SessionCommand{
		Mode:           command.ModeContinue,
		ConversationID: decision.ConversationID,
		WindowID:       decision.WindowID,
		Input:          decision.Message.Text,
		Backend:        "primary",
		CWD:            stack.cfg.DefaultCWD,
	})
	if err != nil {
		return "", err
	}
	return flowResult.Session.ID, nil
}
