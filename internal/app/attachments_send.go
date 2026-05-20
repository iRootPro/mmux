package app

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"band-tui/internal/domain"
)

type composerSendPayload struct {
	Original        string
	Message         string
	AttachmentPaths []string
	Files           []domain.PostFile
}

func parseComposerSend(raw string) (composerSendPayload, error) {
	payload := composerSendPayload{Original: raw}
	var messageLines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		argText, ok := attachmentCommandArgs(trimmed)
		if !ok {
			messageLines = append(messageLines, line)
			continue
		}
		args, err := splitAttachmentArgs(argText)
		if err != nil {
			return payload, err
		}
		if len(args) == 0 {
			return payload, fmt.Errorf("attach: path is empty")
		}
		payload.AttachmentPaths = append(payload.AttachmentPaths, args...)
	}
	payload.Message = strings.TrimSpace(strings.Join(messageLines, "\n"))
	for _, path := range payload.AttachmentPaths {
		file, err := attachmentFileForPath(path)
		if err != nil {
			return payload, err
		}
		payload.Files = append(payload.Files, file)
	}
	return payload, nil
}

func attachmentCommandArgs(line string) (string, bool) {
	for _, prefix := range []string{"/attach", "/file", "📎"} {
		if line == prefix {
			return "", true
		}
		if strings.HasPrefix(line, prefix+" ") {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	return "", false
}

func splitAttachmentArgs(s string) ([]string, error) {
	var args []string
	var b strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		args = append(args, b.String())
		b.Reset()
	}
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			b.WriteRune(r)
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case ' ', '\t':
			flush()
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("attach: unclosed quote")
	}
	flush()
	for i := range args {
		args[i] = expandAttachmentPath(args[i])
	}
	return args, nil
}

func expandAttachmentPath(path string) string {
	path = os.ExpandEnv(strings.TrimSpace(path))
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}

func attachmentFileForPath(path string) (domain.PostFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return domain.PostFile{}, fmt.Errorf("attach %s: %w", path, err)
	}
	if info.IsDir() {
		return domain.PostFile{}, fmt.Errorf("attach %s: is a directory", path)
	}
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	mimeType := ""
	if ext != "" {
		mimeType = mime.TypeByExtension("." + ext)
	}
	return domain.PostFile{
		Name:      filepath.Base(path),
		Extension: ext,
		MIMEType:  mimeType,
		Size:      info.Size(),
	}, nil
}
