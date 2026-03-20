package discord

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareContent(t *testing.T) {
	adapter, err := NewAdapter(Options{BotToken: "token", RequireMention: true})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	adapter.setBotUserID("12345")

	t.Run("direct_message_allows_plain_text", func(t *testing.T) {
		text, ok := adapter.prepareContent("hello", true)
		if !ok || text != "hello" {
			t.Fatalf("unexpected result: ok=%v text=%q", ok, text)
		}
	})

	t.Run("guild_message_requires_mention", func(t *testing.T) {
		_, ok := adapter.prepareContent("hello", false)
		if ok {
			t.Fatalf("expected guild message without mention to be ignored")
		}
	})

	t.Run("guild_message_strips_mention", func(t *testing.T) {
		text, ok := adapter.prepareContent("<@12345> /new", false)
		if !ok || text != "/new" {
			t.Fatalf("unexpected result: ok=%v text=%q", ok, text)
		}
	})
}

func TestSendTextPostsToDiscordAPI(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotBody string

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			gotBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":"1"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	adapter, err := NewAdapter(Options{
		BotToken:   "token-123",
		APIBaseURL: "https://discord.test/api/v10",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	adapter.BindSession("sess-1", Target{ChannelID: "chan-9"})

	if err := adapter.SendText(context.Background(), "sess-1", "hello discord", true); err != nil {
		t.Fatalf("send text: %v", err)
	}

	if gotAuth != "Bot token-123" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotPath != "/api/v10/channels/chan-9/messages" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if !strings.Contains(gotBody, `"content":"hello discord"`) {
		t.Fatalf("unexpected body: %q", gotBody)
	}
}

func TestSendTypingPostsToDiscordAPI(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotMethod string

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			gotMethod = r.Method
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	adapter, err := NewAdapter(Options{
		BotToken:   "token-xyz",
		APIBaseURL: "https://discord.test/api/v10",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	if err := adapter.SendTyping(context.Background(), Target{ChannelID: "chan-typing"}); err != nil {
		t.Fatalf("send typing: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("unexpected method: %q", gotMethod)
	}
	if gotAuth != "Bot token-xyz" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotPath != "/api/v10/channels/chan-typing/typing" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
}

func TestSendDirectUsesInteractionCallback(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody string

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			gotBody = string(body)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	adapter, err := NewAdapter(Options{
		BotToken:   "token-abc",
		APIBaseURL: "https://discord.test/api/v10",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	err = adapter.SendDirect(context.Background(), Target{
		ChannelID:        "chan-1",
		InteractionID:    "inter-1",
		InteractionToken: "inter-token",
	}, "hello interaction")
	if err != nil {
		t.Fatalf("send direct: %v", err)
	}

	if gotPath != "/api/v10/interactions/inter-1/inter-token/callback" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if gotAuth != "" {
		t.Fatalf("interaction callback should not send bot auth header")
	}
	if !strings.Contains(gotBody, `"type":4`) || !strings.Contains(gotBody, `"content":"hello interaction"`) {
		t.Fatalf("unexpected body: %q", gotBody)
	}
}

func TestInteractionToCommandText(t *testing.T) {
	got, ok, err := interactionToCommandText(discordInteractionData{Name: "new"})
	if err != nil || !ok || got != "/new" {
		t.Fatalf("unexpected /new mapping: got=%q ok=%v err=%v", got, ok, err)
	}

	raw, _ := json.Marshal("sess-1")
	got, ok, err = interactionToCommandText(discordInteractionData{
		Name: "resume",
		Options: []discordInteractionOption{
			{Name: "session_id", Type: discordApplicationCommandOptionTypeString, Value: raw},
		},
	})
	if err != nil || !ok || got != "/resume sess-1" {
		t.Fatalf("unexpected /resume mapping: got=%q ok=%v err=%v", got, ok, err)
	}

	got, ok, err = interactionToCommandText(discordInteractionData{
		Name: "switch",
		Options: []discordInteractionOption{
			{Name: "session_id", Type: discordApplicationCommandOptionTypeString, Value: raw},
		},
	})
	if err != nil || !ok || got != "/switch sess-1" {
		t.Fatalf("unexpected /switch mapping: got=%q ok=%v err=%v", got, ok, err)
	}

	got, ok, err = interactionToCommandText(discordInteractionData{Name: "clawx-skills"})
	if err != nil || !ok || got != "/clawx-skills" {
		t.Fatalf("unexpected /clawx-skills mapping: got=%q ok=%v err=%v", got, ok, err)
	}

	projectArgsRaw, _ := json.Marshal("use image_tools")
	got, ok, err = interactionToCommandText(discordInteractionData{
		Name: "project",
		Options: []discordInteractionOption{
			{Name: "args", Type: discordApplicationCommandOptionTypeString, Value: projectArgsRaw},
		},
	})
	if err != nil || !ok || got != "/project use image_tools" {
		t.Fatalf("unexpected /project mapping: got=%q ok=%v err=%v", got, ok, err)
	}

	memoryArgsRaw, _ := json.Marshal("audit")
	got, ok, err = interactionToCommandText(discordInteractionData{
		Name: "memory",
		Options: []discordInteractionOption{
			{Name: "args", Type: discordApplicationCommandOptionTypeString, Value: memoryArgsRaw},
		},
	})
	if err != nil || !ok || got != "/memory audit" {
		t.Fatalf("unexpected /memory mapping: got=%q ok=%v err=%v", got, ok, err)
	}

	nameRaw, _ := json.Marshal("echo")
	inputRaw, _ := json.Marshal("hello")
	got, ok, err = interactionToCommandText(discordInteractionData{
		Name: "sx-skill",
		Options: []discordInteractionOption{
			{Name: "name", Type: discordApplicationCommandOptionTypeString, Value: nameRaw},
			{Name: "input", Type: discordApplicationCommandOptionTypeString, Value: inputRaw},
		},
	})
	if err != nil || !ok || got != "/sx-skill echo hello" {
		t.Fatalf("unexpected /sx-skill mapping: got=%q ok=%v err=%v", got, ok, err)
	}
}

func TestNormalizeMessageIncludesAttachments(t *testing.T) {
	adapter, err := NewAdapter(Options{BotToken: "token"})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	envelope, ok, err := adapter.normalizeMessage(discordMessageCreate{
		ChannelID: "chan-1",
		Content:   "转换成长图",
		Attachments: []discordAttachment{
			{
				Filename:    "test.pdf",
				URL:         "https://cdn.discordapp.com/test.pdf",
				ContentType: "application/pdf",
				Size:        9195520,
			},
		},
		Author: struct {
			ID  string `json:"id"`
			Bot bool   `json:"bot"`
		}{
			ID: "user-1",
		},
	})
	if err != nil {
		t.Fatalf("normalize message: %v", err)
	}
	if !ok {
		t.Fatalf("expected message to be accepted")
	}
	if got := len(envelope.Message.Attachments); got != 1 {
		t.Fatalf("unexpected attachments count: %d", got)
	}
	if envelope.Message.Attachments[0].Name != "test.pdf" {
		t.Fatalf("unexpected attachment name: %q", envelope.Message.Attachments[0].Name)
	}
	if envelope.Message.Attachments[0].ContentType != "application/pdf" {
		t.Fatalf("unexpected attachment content type: %q", envelope.Message.Attachments[0].ContentType)
	}
}

func TestNormalizeMessageAcceptsDirectAttachmentOnly(t *testing.T) {
	adapter, err := NewAdapter(Options{BotToken: "token"})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	envelope, ok, err := adapter.normalizeMessage(discordMessageCreate{
		ChannelID: "chan-1",
		Content:   "",
		Attachments: []discordAttachment{
			{
				Filename: "test.pdf",
				URL:      "https://cdn.discordapp.com/test.pdf",
			},
		},
		Author: struct {
			ID  string `json:"id"`
			Bot bool   `json:"bot"`
		}{
			ID: "user-1",
		},
	})
	if err != nil {
		t.Fatalf("normalize attachment-only message: %v", err)
	}
	if !ok {
		t.Fatalf("expected attachment-only direct message to be accepted")
	}
	if strings.TrimSpace(envelope.Message.Text) == "" {
		t.Fatalf("expected synthetic text for attachment-only message")
	}
}

func TestSendLocalFilePostsMultipartToDiscordAPI(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotCT string
	var gotPayload string
	var gotFileName string
	var gotFileBody string

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			gotCT = r.Header.Get("Content-Type")
			if err := r.ParseMultipartForm(2 * 1024 * 1024); err != nil {
				t.Fatalf("parse multipart: %v", err)
			}
			gotPayload = r.FormValue("payload_json")
			file, header, err := r.FormFile("files[0]")
			if err != nil {
				t.Fatalf("read form file: %v", err)
			}
			defer file.Close()
			body, err := io.ReadAll(file)
			if err != nil {
				t.Fatalf("read form file body: %v", err)
			}
			gotFileName = header.Filename
			gotFileBody = string(body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":"1"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	adapter, err := NewAdapter(Options{
		BotToken:   "token-file",
		APIBaseURL: "https://discord.test/api/v10",
		HTTPClient: client,
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-long.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := adapter.SendLocalFile(context.Background(), Target{ChannelID: "chan-file"}, path, "产物回传"); err != nil {
		t.Fatalf("send local file: %v", err)
	}

	if gotAuth != "Bot token-file" {
		t.Fatalf("unexpected auth header: %q", gotAuth)
	}
	if gotPath != "/api/v10/channels/chan-file/messages" {
		t.Fatalf("unexpected path: %q", gotPath)
	}
	if !strings.HasPrefix(strings.ToLower(gotCT), "multipart/form-data;") {
		t.Fatalf("unexpected content type: %q", gotCT)
	}
	if !strings.Contains(gotPayload, `"filename":"test-long.png"`) {
		t.Fatalf("unexpected payload_json: %q", gotPayload)
	}
	if gotFileName != "test-long.png" {
		t.Fatalf("unexpected file name: %q", gotFileName)
	}
	if gotFileBody != "png-bytes" {
		t.Fatalf("unexpected file body: %q", gotFileBody)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
