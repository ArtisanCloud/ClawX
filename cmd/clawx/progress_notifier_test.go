package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestConversationProgressNotifierEmit(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_TIMEOUT", "1s")

	ch := make(chan string, 1)
	registerConversationProgressNotifier("conv-progress-1", func(_ context.Context, message string) error {
		ch <- message
		return nil
	})

	emitConversationProgressNotification("conv-progress-1", "进度更新 A")

	select {
	case got := <-ch:
		if got != "进度更新 A" {
			t.Fatalf("unexpected notify message: %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected progress notification emitted")
	}
}

func TestConversationProgressNotifierDedupDigest(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")

	var count int32
	done := make(chan struct{}, 2)
	registerConversationProgressNotifier("conv-progress-2", func(_ context.Context, _ string) error {
		atomic.AddInt32(&count, 1)
		done <- struct{}{}
		return nil
	})

	emitConversationProgressNotification("conv-progress-2", "同一条消息")
	emitConversationProgressNotification("conv-progress-2", "同一条消息")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected at least one notification")
	}
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("expected deduped notification count=1, got=%d", got)
	}
}

func TestConversationProgressNotifierMinInterval(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "1h")

	var count int32
	done := make(chan struct{}, 2)
	registerConversationProgressNotifier("conv-progress-3", func(_ context.Context, _ string) error {
		atomic.AddInt32(&count, 1)
		done <- struct{}{}
		return nil
	})

	emitConversationProgressNotification("conv-progress-3", "进度更新 A")
	emitConversationProgressNotification("conv-progress-3", "进度更新 B")

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected first notification")
	}
	time.Sleep(80 * time.Millisecond)
	if got := atomic.LoadInt32(&count); got != 1 {
		t.Fatalf("expected min interval gate to suppress second notification, got=%d", got)
	}
}

func TestConversationProgressNotifierPersistedRouteAfterReset(t *testing.T) {
	resetConversationProgressNotifierForTest()
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_TIMEOUT", "1s")
	routeFile := filepath.Join(t.TempDir(), "conversation_progress_routes.json")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_ROUTE_FILE", routeFile)

	type fakeTarget struct {
		ChannelID string `json:"channel_id"`
		UserID    string `json:"user_id"`
	}

	conversationID := "conv-progress-persist-1"
	target := fakeTarget{
		ChannelID: "ch-1",
		UserID:    "u-1",
	}

	first := make(chan string, 1)
	registerConversationProgressDispatcher("discord", "discord-default", func(_ context.Context, targetRaw json.RawMessage, message string) error {
		var decoded fakeTarget
		if err := json.Unmarshal(targetRaw, &decoded); err != nil {
			return err
		}
		if decoded != target {
			return fmt.Errorf("unexpected target: %+v", decoded)
		}
		first <- message
		return nil
	})
	bindConversationProgressRoute(conversationID, "discord", "discord-default", target)
	emitConversationProgressNotification(conversationID, "进度更新 A")

	select {
	case got := <-first:
		if got != "进度更新 A" {
			t.Fatalf("unexpected first route message: %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected first route notification")
	}

	resetConversationProgressNotifierForTest()
	t.Setenv("CLAWX_PROGRESS_NOTIFY_DISABLED", "0")
	t.Setenv("CLAWX_PROGRESS_NOTIFY_MIN_INTERVAL", "0")

	second := make(chan string, 1)
	registerConversationProgressDispatcher("discord", "discord-default", func(_ context.Context, targetRaw json.RawMessage, message string) error {
		var decoded fakeTarget
		if err := json.Unmarshal(targetRaw, &decoded); err != nil {
			return err
		}
		if decoded != target {
			return fmt.Errorf("unexpected target after reset: %+v", decoded)
		}
		second <- message
		return nil
	})
	emitConversationProgressNotification(conversationID, "进度更新 B")

	select {
	case got := <-second:
		if got != "进度更新 B" {
			t.Fatalf("unexpected second route message: %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected persisted route notification after reset")
	}

	body, err := os.ReadFile(routeFile)
	if err != nil {
		t.Fatalf("read route file: %v", err)
	}
	var snapshot conversationProgressRouteSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("decode route file: %v", err)
	}
	found := false
	for _, route := range snapshot.Routes {
		if route.ConversationID == conversationID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected route persisted for conversation %s", conversationID)
	}
}

func resetConversationProgressNotifierForTest() {
	globalConversationProgressNotifiers.mu.Lock()
	defer globalConversationProgressNotifiers.mu.Unlock()
	globalConversationProgressNotifiers.byConv = make(map[string]conversationProgressNotifier)
	globalConversationProgressNotifiers.dispatchers = make(map[string]conversationProgressDispatcher)
	globalConversationProgressNotifiers.routes = make(map[string]conversationProgressRoute)
	globalConversationProgressNotifiers.state = make(map[string]conversationProgressState)
	globalConversationProgressNotifiers.routesLoaded = false
}
