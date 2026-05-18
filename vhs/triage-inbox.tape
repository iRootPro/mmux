# band-tui triage inbox smoke recording
# Run from repo root with: vhs vhs/triage-inbox.tape

Output tmp/triage-inbox.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 25ms

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s

# Move focus out of the composer so printable shortcuts are global.
Tab
Sleep 500ms

# Open triage, move selection, and open the selected unread channel.
Type "u"
Sleep 1s
Down
Sleep 500ms
Enter
Sleep 1s

# Reopen triage, locally dismiss the remaining item, then close.
Type "u"
Sleep 700ms
Type "d"
Sleep 700ms
Escape
Sleep 500ms

Ctrl+C
