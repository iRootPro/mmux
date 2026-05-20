# mmux thread modal UX walkthrough
# Run from repo root with: vhs vhs/thread-modal-ux.tape

Output tmp/thread-modal-ux.gif

Set Shell "bash"
Set FontSize 14
Set Width 1600
Set Height 1000
Set TypingSpeed 20ms

Type "./tmp/mmux-vhs --mock"
Enter
Sleep 2s

# Leave compose mode and inspect timeline.
Tab
Tab
Sleep 700ms
Down
Down
Sleep 500ms

# Reply flow: r should open thread modal and composer should be active immediately.
Type "r"
Sleep 1s
Type "ответ в тред без tab"
Sleep 900ms
Enter
Sleep 1s

# Switch to thread messages only when explicitly needed, scroll, then back to reply.
Tab
Sleep 400ms
Up
Down
Sleep 500ms
Tab
Sleep 400ms
Type "ещё ответ"
Sleep 700ms
Enter
Sleep 1s

# Attach command in the modal composer.
Type "/attach README.md"
Sleep 700ms
Enter
Sleep 1s

# Close modal, then open another thread via t.
Escape
Sleep 700ms
Down
Down
Type "t"
Sleep 1s
Escape
Sleep 700ms

# Triage: open inbox globally, open first item, close.
Type "u"
Sleep 700ms
Enter
Sleep 1s
Escape
Sleep 500ms

Ctrl+C
