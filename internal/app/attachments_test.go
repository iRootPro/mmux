package app

import (
	"strings"
	"testing"

	"band-tui/internal/domain"
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
