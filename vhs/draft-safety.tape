# band-tui draft safety smoke recording
# Run from repo root with: vhs vhs/draft-safety.tape

Output tmp/draft-safety.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 25ms

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s

# Type a channel draft, switch away, return, and verify visually it remains.
Type "dev draft"
Sleep 500ms
Tab
Sleep 300ms
Down
Sleep 800ms
Up
Sleep 800ms
Tab
Tab
Sleep 500ms

# Trigger deterministic send failure and show restored text.
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Type "fail-send"
Enter
Sleep 1s

Ctrl+C
