# TUI Demo — a developer's morning check-in
#
# Note: Ctrl+T/F/K/X are secondary bindings for Alt+T/F/C/X.
#       VHS cannot send Alt key combos (sends ESC + char separately).
#
# Warning: Requires an unmerged commit 
# https://github.com/mikecsmith/vhs/commit/4e5e30252a72252947b0de4d7ceda9429a66fa19
# on a fork to run with custom MouseScroll lines

Output vhs/output/tui-demo.mp4

Require ihj

Set Shell "bash"
Set FontFamily "MesloLGLDZ Nerd Font"
Set FontSize 14
Set Width 1400
Set Height 800
Set Padding 10
Set Theme "tokyonight"
Set TypingSpeed 120ms
Set Framerate 30
Set MouseScrollLines 1
Set MouseScrollDelay 50ms

# ── Open the board ──────────────────────────────────────────────
Hide
Type "mise exec -- ihj jira demo"
Enter
Sleep 3s
Show

# Scan the board briefly
Sleep 2s

# ── Filter to my tickets ──────────────────────────────────────
Ctrl+F
Sleep 1s
Down
Sleep 400ms
Enter
Sleep 2s

# ── Quick task: move finished work to review ───────────────────
# DEMO-4 "Write unit tests" is In Progress — finished last night
Down
Sleep 1s

# Transition to In Review (Backlog → To Do → In Progress → ▸ In Review)
Ctrl+T
Sleep 1s
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms
Enter
Sleep 1.5s

# ── Switch back to the full active board ───────────────────────
Ctrl+F
Sleep 1s
Down
Sleep 400ms
Enter
Sleep 2s

# ── Browse the auth epic ──────────────────────────────────────
# Search to find the auth tickets
Type "auth"
Sleep 1.5s

# DEMO-1 (epic) is selected — read the top of the preview
Sleep 2s

# Scroll past the overview to the goals section
MouseScrollDown 8 @100,10

# Read the goals
Sleep 3s

# Scroll to out of scope / child issues
MouseScrollDown 10 @100,10

# Read child issues list
Sleep 3s

# Scroll to the comments
MouseScrollDown 12 @100,10

# Read the comments
Sleep 3s

# ── Review the OAuth PKCE story ────────────────────────────────
# Move down to DEMO-2 (the story with the codeblock)
Down
Sleep 2.5s

# Scroll past header to acceptance criteria
MouseScrollDown 8 @100,10

# Read the acceptance criteria
Sleep 3.5s

# Scroll to the technical notes
MouseScrollDown 6 @100,10

# Read the technical notes intro
Sleep 2.5s

# Scroll the full codeblock into view (8 lines of code + border + a line below)
MouseScrollDown 12 @100,10

# Read the PKCE codeblock — this is the key part
Sleep 4s

# Scroll past to child issues
MouseScrollDown 10 @100,10

# Glance at child issues
Sleep 2s

# Scroll to the comment thread
MouseScrollDown 8 @100,10

# Read the first comment
Sleep 3s

# Scroll to the remaining comments
MouseScrollDown 12 @100,10

# Read through the rest of the conversation
Sleep 3s

# ── Leave review feedback on DEMO-2 ──────────────────────────
Ctrl+K
Sleep 1.5s

Type "Looks good — S256 challenge method is correct per the RFC. One thought: should we add rate limiting on the token endpoint before this ships?"
Sleep 2s

Ctrl+S
Sleep 1s

# Scroll down to see the comment rendered in
MouseScrollDown 8 @100,10

# See the comment
Sleep 2.5s

# ── Check own epic's progress ─────────────────────────────────
# Clear search and find the API performance epic
Backspace
Sleep 150ms
Backspace
Sleep 150ms
Backspace
Sleep 150ms
Backspace
Sleep 500ms

Type "API perf"
Sleep 1.5s

# DEMO-10 should be selected — drill into children for a quick scan
Enter
Sleep 1.5s

# Scan — back out
Escape
Sleep 1.5s

# ── Extract epic context for LLM planning ─────────────────────
Ctrl+X
Sleep 1s

# Select "Selected + children" (2nd option)
Down
Sleep 300ms
Enter
Sleep 1.5s

# Write a prompt for the LLM
Type "Help me refine these API performance tickets. I'll paste a transcript from last week's architecture review — pay close attention to the discussion around nonfunctional requirements (latency SLOs, cache TTLs, connection pool limits) and add them to the relevant tickets. Ask me questions before making changes."
Sleep 2.5s

Ctrl+S
Sleep 2s

# ── Exit and hand off to the LLM ──────────────────────────────
Ctrl+C
Hide
Sleep 100ms
Type "clear"
Enter
Sleep 500ms
Show
Sleep 500ms

# Paste the extracted context into Claude for an interactive session
Type "pbpaste | claude"
Sleep 2s
