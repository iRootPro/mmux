package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"band-tui/internal/domain"
	"band-tui/internal/mock"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestRenderPostsShowsFileOnlyMessage(t *testing.T) {
	m := Model{
		viewport: viewport.Model{Width: 100},
		posts: []domain.Post{{
			ID:       "p1",
			Username: "Alice",
			CreateAt: 1770000000000,
			Files: []domain.PostFile{{
				ID:       "file-id-1",
				Name:     "photo.png",
				MIMEType: "image/png",
				Size:     1536,
				Width:    800,
				Height:   600,
			}},
		}},
	}

	got, _ := m.renderPosts()
	if !strings.Contains(got, "Alice") || !strings.Contains(got, "photo.png") || !strings.Contains(got, "image/png") || !strings.Contains(got, "800×600") {
		t.Fatalf("file-only post not rendered with attachment details:\n%s", got)
	}
}

func TestFormatQuotedReplyIncludesAttachments(t *testing.T) {
	post := domain.Post{Username: "Alice", Message: "see this", Files: []domain.PostFile{{Name: "report.pdf", MIMEType: "application/pdf", Size: 2048}}}
	got := formatQuotedReply(post)
	if !strings.Contains(got, "> see this") || !strings.Contains(got, "> 📎 report.pdf (2.0 KB · application/pdf)") {
		t.Fatalf("quote missing attachment:\n%q", got)
	}
}

func TestParseComposerSendExtractsAttachLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "report final.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, err := parseComposerSend("please see\n/attach \"" + file + "\"\nthanks")
	if err != nil {
		t.Fatal(err)
	}
	if payload.Message != "please see\nthanks" {
		t.Fatalf("message = %q", payload.Message)
	}
	if len(payload.AttachmentPaths) != 1 || payload.AttachmentPaths[0] != file {
		t.Fatalf("paths = %#v", payload.AttachmentPaths)
	}
	if len(payload.Files) != 1 || payload.Files[0].Name != "report final.txt" || payload.Files[0].Size != 5 {
		t.Fatalf("files = %#v", payload.Files)
	}
}

func TestSendPostWithAttachLineShowsAttachmentAndStripsCommand(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(mock.New(), testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("attached\n/attach " + file)

	updated, cmd := m.handleKey(draftKey("enter"))
	got := updated.(Model)
	if len(got.posts) != 1 || len(got.posts[0].Files) != 1 || got.posts[0].Message != "attached" {
		t.Fatalf("pending attachment post wrong: %#v", got.posts)
	}
	msg := cmd()
	postMsg, ok := msg.(postSentMsg)
	if !ok || postMsg.err != nil {
		t.Fatalf("send msg = %#v", msg)
	}
	updated, _ = got.Update(postMsg)
	got = updated.(Model)
	if len(got.posts) != 1 || got.posts[0].Message != "attached" || len(got.posts[0].Files) != 1 || got.posts[0].Files[0].Name != "note.txt" {
		t.Fatalf("delivered attachment post wrong: %#v", got.posts)
	}
}

func TestAttachMissingFileKeepsDraft(t *testing.T) {
	m := New(mock.New(), testConfig(), false)
	m.channels = []domain.Channel{{ID: "dev", Type: "O", DisplayName: "dev"}}
	m.selectedChannel = 0
	m.focus = focusComposer
	m.loadDraft(channelDraftKey("dev"))
	m.composer.SetValue("/attach /no/such/file")

	updated, cmd := m.handleKey(draftKey("enter"))
	got := updated.(Model)
	if cmd != nil || len(got.posts) != 0 || got.composer.Value() != "/attach /no/such/file" || !strings.Contains(got.status, "attachment failed") {
		t.Fatalf("missing attachment should not send: status=%q posts=%#v composer=%q", got.status, got.posts, got.composer.Value())
	}
}
