package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

func TestAttachmentContextStoreResolveReuseByRoute(t *testing.T) {
	root := t.TempDir()
	store, err := newAttachmentContextStore(config.Snapshot{
		Projects: config.ProjectConfig{WorkspaceRoot: filepath.Join(root, "workspaces")},
	})
	if err != nil {
		t.Fatalf("new attachment context store: %v", err)
	}

	route := "discord:default:direct:u1:thread:ch1"
	first := []chatiface.Attachment{
		{
			Name:        "test.pdf",
			URL:         "https://cdn.discordapp.com/test.pdf",
			ContentType: "application/pdf",
			SizeBytes:   1234,
		},
	}
	got, err := store.Resolve("image_tools", "main", route, first)
	if err != nil {
		t.Fatalf("save resolve: %v", err)
	}
	if len(got) != 1 || got[0].Name != "test.pdf" {
		t.Fatalf("unexpected first resolve result: %#v", got)
	}

	reused, err := store.Resolve("image_tools", "main", route, nil)
	if err != nil {
		t.Fatalf("reuse resolve: %v", err)
	}
	if len(reused) != 1 || reused[0].Name != "test.pdf" {
		t.Fatalf("unexpected reused attachments: %#v", reused)
	}
}

func TestAttachmentContextStoreRouteIsolation(t *testing.T) {
	root := t.TempDir()
	store, err := newAttachmentContextStore(config.Snapshot{
		Projects: config.ProjectConfig{WorkspaceRoot: filepath.Join(root, "workspaces")},
	})
	if err != nil {
		t.Fatalf("new attachment context store: %v", err)
	}

	if _, err := store.Resolve("image_tools", "main", "discord:default:direct:u1:thread:a", []chatiface.Attachment{
		{Name: "a.pdf", URL: "https://cdn.discordapp.com/a.pdf"},
	}); err != nil {
		t.Fatalf("seed route a: %v", err)
	}

	got, err := store.Resolve("image_tools", "main", "discord:default:direct:u1:thread:b", nil)
	if err != nil {
		t.Fatalf("resolve route b: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("route isolation broken, got=%#v", got)
	}
}

func TestAttachmentContextStoreDownloadsLocalCopyAndReuses(t *testing.T) {
	root := t.TempDir()
	store, err := newAttachmentContextStore(config.Snapshot{
		Projects: config.ProjectConfig{WorkspaceRoot: filepath.Join(root, "workspaces")},
	})
	if err != nil {
		t.Fatalf("new attachment context store: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pdf-content"))
	}))
	defer server.Close()

	route := "discord:default:direct:u1:thread:ch1"
	first, err := store.Resolve("image_tools", "main", route, []chatiface.Attachment{
		{
			Name: "test.pdf",
			URL:  server.URL + "/test.pdf",
		},
	})
	if err != nil {
		t.Fatalf("resolve with remote attachment: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("unexpected attachment count: %d", len(first))
	}
	if first[0].LocalPath == "" {
		t.Fatalf("expected local path to be materialized")
	}
	body, err := os.ReadFile(first[0].LocalPath)
	if err != nil {
		t.Fatalf("read local attachment copy: %v", err)
	}
	if string(body) != "pdf-content" {
		t.Fatalf("unexpected local attachment body: %q", string(body))
	}

	reused, err := store.Resolve("image_tools", "main", route, nil)
	if err != nil {
		t.Fatalf("resolve cached attachment: %v", err)
	}
	if len(reused) != 1 {
		t.Fatalf("unexpected cached attachment count: %d", len(reused))
	}
	if reused[0].LocalPath != first[0].LocalPath {
		t.Fatalf("expected stable local path, got=%q want=%q", reused[0].LocalPath, first[0].LocalPath)
	}
}
