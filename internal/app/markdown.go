package app

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

var (
	markdownLinkRE = regexp.MustCompile(`\[([^\]\n]+)\]\(([^\s\)]+)\)`)
	inlineCodeRE   = regexp.MustCompile("`([^`\n]+)`")
	boldRE         = regexp.MustCompile(`\*\*([^*\n]+)\*\*`)
	italicRE       = regexp.MustCompile(`(^|\s)\*([^*\n]+)\*`)
	bareURLRE      = regexp.MustCompile(`https?://[^\s<>]+`)
	mentionRE      = regexp.MustCompile(`(^|\s)([@#][A-Za-z0-9._-]+)`)

	inlineCodeStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "250"}).Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})
	codeBlockStyle  = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240", Dark: "250"})
	quoteBarStyle   = lipgloss.NewStyle().Foreground(colorAccent)
	// Avoid terminal underline artifacts on long wrapped URLs; accent color is enough
	// to distinguish links without drawing horizontal lines through message blocks.
	linkStyle = lipgloss.NewStyle().Foreground(colorAccent)
)

func renderMarkdownMessage(message string, width int) string {
	_ = width
	message = strings.TrimRight(sanitizeMessageText(message), "\n")
	if message == "" {
		return ""
	}

	var out []string
	inFence := false
	for _, line := range strings.Split(message, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			out = append(out, codeBlockStyle.Render("  "+line))
			continue
		}
		out = append(out, renderMarkdownLine(line))
	}
	rendered := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if width > 0 {
		rendered = wordwrap.String(rendered, width)
	}
	return strings.TrimRight(rendered, "\n")
}

func renderMarkdownLine(line string) string {
	trimmed := strings.TrimSpace(line)
	leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	if strings.HasPrefix(trimmed, ">") {
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		return quoteBarStyle.Render("│ ") + muted.Render(renderInlineMarkdown(body))
	}

	if strings.HasPrefix(trimmed, "#") {
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level > 0 && level < len(trimmed) && trimmed[level] == ' ' {
			return accent.Bold(true).Render(strings.TrimSpace(trimmed[level:]))
		}
	}

	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return leading + accent.Render("• ") + renderInlineMarkdown(strings.TrimSpace(trimmed[2:]))
	}

	return renderInlineMarkdown(line)
}

func renderInlineMarkdown(s string) string {
	s = markdownLinkRE.ReplaceAllStringFunc(s, func(match string) string {
		parts := markdownLinkRE.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		text := parts[1]
		url := parts[2]
		if text == url {
			return linkStyle.Render(url)
		}
		return linkStyle.Render(text) + muted.Render(" <"+url+">")
	})
	if !strings.Contains(s, "](") {
		s = bareURLRE.ReplaceAllStringFunc(s, func(match string) string {
			return linkStyle.Render(match)
		})
	}
	s = inlineCodeRE.ReplaceAllString(s, inlineCodeStyle.Render(" $1 "))
	s = boldRE.ReplaceAllStringFunc(s, func(match string) string {
		parts := boldRE.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		return lipgloss.NewStyle().Bold(true).Render(parts[1])
	})
	s = italicRE.ReplaceAllStringFunc(s, func(match string) string {
		parts := italicRE.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		return parts[1] + lipgloss.NewStyle().Italic(true).Render(parts[2])
	})
	s = mentionRE.ReplaceAllStringFunc(s, func(match string) string {
		parts := mentionRE.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		return parts[1] + accent.Render(parts[2])
	})
	return s
}

func sanitizeMessageText(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\x1b' || r == '\u009b' {
			return -1
		}
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 32 || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
}
