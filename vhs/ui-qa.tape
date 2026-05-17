# band-tui UI/UX smoke recording
# Run from repo root with: vhs vhs/ui-qa.tape

Output tmp/band-tui-ui-qa.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 25ms

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s

# Composer focus: printable shortcuts should type into the composer.
Type "/at["
Sleep 1s
Backspace
Backspace
Backspace
Backspace
Sleep 500ms

# Sidebar focus: composer becomes inactive and sidebar hints appear.
Tab
Sleep 900ms

# Sidebar shortcut: / opens channel filter only outside composer.
Type "/"
Sleep 700ms
Escape
Sleep 500ms

# Timeline focus: selected message marker and timeline hints should appear.
Tab
Sleep 900ms
Down
Down
Sleep 700ms

# Open a thread from the selected timeline post.
Type "t"
Sleep 2s

# Thread messages focus: reply composer should be inactive and status should say thread messages.
Sleep 1s

# Thread reply focus: reply composer becomes active; text routes into the reply input.
Tab
Sleep 900ms
Type "qa reply"
Sleep 900ms
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Backspace
Sleep 500ms

# Return to thread messages, then close the thread.
Tab
Sleep 800ms
Escape
Sleep 800ms

# Quit regardless of current focus.
Ctrl+C
