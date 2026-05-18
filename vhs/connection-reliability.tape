# band-tui connection reliability smoke
# Run from repo root with: vhs vhs/connection-reliability.tape

Output tmp/connection-reliability.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 25ms

Type "go run ./cmd/band-tui --mock"
Enter
Sleep 3s

Type "mock:offline"
Enter
Sleep 1s

Type "mock:reconnect"
Enter
Sleep 1s

Type "mock:auth-expired"
Enter
Sleep 1s

Ctrl+C
