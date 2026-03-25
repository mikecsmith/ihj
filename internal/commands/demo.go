package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mikecsmith/ihj/internal/config"
	"github.com/mikecsmith/ihj/internal/document"
	"github.com/mikecsmith/ihj/internal/jira"
)

// RunDemo launches the TUI with synthetic data — no Jira connection needed.
func RunDemo(app *App) error {
	if app.LaunchTUI == nil {
		return fmt.Errorf("TUI not available (LaunchTUI not configured)")
	}

	board := demoBoard()
	views := demoIssues()

	// Wire up parent/child relationships.
	registry := make(map[string]*jira.IssueView, len(views))
	for i := range views {
		registry[views[i].Key] = &views[i]
	}
	jira.LinkChildren(registry)

	flat := make([]jira.IssueView, 0, len(registry))
	for _, v := range registry {
		flat = append(flat, *v)
	}

	// Convert IssueViews into jira.Issue records so the MockClient can
	// look them up for transitions, comments, assignments, edits, etc.
	clientIssues := make([]jira.Issue, 0, len(flat))
	for _, v := range flat {
		iss := jira.Issue{
			Key: v.Key,
			Fields: jira.IssueFields{
				Summary:   v.Summary,
				Status:    jira.Status{Name: v.Status},
				Priority:  jira.Priority{Name: v.Priority},
				IssueType: jira.IssueType{Name: v.Type},
				Created:   v.Created,
				Updated:   v.Updated,
			},
		}
		// Convert the AST description back to ADF JSON so edit mode can
		// round-trip it: ADF → AST → Markdown (for editor) → AST → ADF.
		if v.Desc != nil {
			adfMap := jira.RenderADFValue(v.Desc)
			if raw, err := json.Marshal(adfMap); err == nil {
				iss.Fields.Description = raw
			}
		}
		// Preserve parent reference for sub-issues.
		if v.ParentKey != "" {
			iss.Fields.Parent = &jira.ParentRef{Key: v.ParentKey}
		}
		clientIssues = append(clientIssues, iss)
	}

	// Register the demo board in the config so Upsert can resolve it.
	if app.Config.Boards == nil {
		app.Config.Boards = make(map[string]*config.BoardConfig)
	}
	app.Config.Boards[board.Slug] = board
	app.Config.DefaultBoard = board.Slug

	// Provide a minimal CustomFields map so frontmatter includes team: true.
	if app.Config.CustomFields == nil {
		app.Config.CustomFields = map[string]int{"team": 15000}
	}

	// Install a MockClient so TUI actions (transition, comment, assign) work
	// against in-memory data without a Jira connection.
	mock := jira.NewMockClient(clientIssues, board.Transitions, board.ProjectKey)
	mock.Latency = 150 * time.Millisecond // Simulate real API latency.
	app.Client = mock

	return app.LaunchTUI(&LaunchTUIData{
		App:    app,
		Board:  board,
		Filter: "active",
		Views:  flat,
	})
}

func demoBoard() *config.BoardConfig {
	board := &config.BoardConfig{
		ID:         99999,
		Name:       "Demo Board",
		Slug:       "demo",
		ProjectKey: "DEMO",
		Transitions: []string{
			"Backlog", "To Do", "In Progress", "In Review", "Done",
		},
		Types: []config.IssueTypeConfig{
			{ID: 1, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
			{ID: 2, Name: "Story", Order: 30, Color: "cyan", HasChildren: true},
			{ID: 3, Name: "Task", Order: 30, Color: "default", HasChildren: true},
			{ID: 4, Name: "Bug", Order: 30, Color: "red", HasChildren: false},
			{ID: 5, Name: "Sub-task", Order: 40, Color: "white", HasChildren: false},
		},
		TypeOrderMap: make(map[string]config.TypeOrderEntry),
	}

	for _, t := range board.Types {
		board.TypeOrderMap[fmt.Sprintf("%d", t.ID)] = config.TypeOrderEntry{
			Order: t.Order, Color: t.Color, HasChildren: t.HasChildren,
		}
	}

	return board
}

func demoIssues() []jira.IssueView {
	now := time.Now()
	d := func(days int) string {
		return now.AddDate(0, 0, -days).Format("02 Jan 2006")
	}
	dt := func(days, hours int) string {
		return now.AddDate(0, 0, -days).Add(time.Duration(-hours) * time.Hour).Format("02 Jan 2006, 15:04")
	}

	md := func(text string) *document.Node {
		doc, _ := document.ParseMarkdownString(text)
		return doc
	}

	return []jira.IssueView{
		// ── Epic 1: Authentication ──────────────────────────────────
		{
			Key: "DEMO-1", Summary: "User Authentication Overhaul",
			Type: "Epic", TypeID: "1", Status: "In Progress", Priority: "High",
			Assignee: "Sarah Chen", Reporter: "Mike Smith",
			Created: d(45), Updated: d(2),
			Labels: "security, q1-priority", Components: "Auth",
			Desc: md("## Overview\n\nReplace the legacy session-based authentication with **OAuth 2.0 + PKCE** flow.\n\n## Goals\n\n- Eliminate session token storage compliance issues\n- Support SSO via SAML/OIDC\n- Reduce login friction by 40%\n\n## Out of Scope\n\n- Migration of existing user sessions (handled by DEMO-15)\n- Mobile app auth (separate epic)"),
			Comments: []jira.CommentView{
				{Author: "Mike Smith", Created: dt(30, 3),
					Body: md("Kicked off the epic. Sarah is leading this — let's aim to have the PKCE flow in staging by end of sprint 4.")},
				{Author: "Sarah Chen", Created: dt(14, 2),
					Body: md("Quick update: PKCE implementation is **on track**. The admin panel (DEMO-3) might slip to next sprint due to design dependency.\n\n> The IdP error handling (DEMO-20) is a pre-existing issue we should fix in parallel.\n\n_— Sarah_")},
			},
			Children: make(map[string]*jira.IssueView),
		},
		// Stories under Epic 1
		{
			Key: "DEMO-2", Summary: "Implement OAuth 2.0 PKCE login flow",
			Type: "Story", TypeID: "2", Status: "In Review", Priority: "High",
			Assignee: "Alex Rivera", Reporter: "Sarah Chen",
			Created: d(30), Updated: d(1), ParentKey: "DEMO-1",
			Labels: "security", Components: "Auth, API",
			Desc: md("Implement the full OAuth 2.0 Authorization Code flow with PKCE.\n\n## Acceptance Criteria\n\n- Given a user visits the login page, when they click Sign In, then they are redirected to the IdP\n- Given the IdP returns an auth code, when the callback fires, then a valid access token is issued\n- Given the PKCE verifier is missing, when the token exchange occurs, then the request is rejected with 400\n\n## Technical Notes\n\nUse `S256` for the code challenge method. Plain is deprecated per [RFC 7636](https://tools.ietf.org/html/rfc7636).\n\n```go\nfunc generatePKCE() (verifier, challenge string) {\n    b := make([]byte, 32)\n    rand.Read(b)\n    verifier = base64.RawURLEncoding.EncodeToString(b)\n    h := sha256.Sum256([]byte(verifier))\n    challenge = base64.RawURLEncoding.EncodeToString(h[:])\n    return\n}\n```"),
			Comments: []jira.CommentView{
				{Author: "Alex Rivera", Created: dt(10, 5),
					Body: md("Started the implementation. The PKCE challenge generation is working. Still need to wire up the callback handler.")},
				{Author: "Sarah Chen", Created: dt(3, 8),
					Body: md("Looks great so far. Can we make sure we're using `S256` for the code challenge method? Plain is deprecated.")},
				{Author: "Alex Rivera", Created: dt(1, 2),
					Body: md("Done — PR is up for review. Key changes:\n\n1. Added PKCE challenge/verifier generation\n2. Updated the `/auth/callback` handler\n3. Added integration tests against the mock IdP\n\nThe **token exchange** is fully tested. @Sarah can you review when you get a chance?")},
			},
			Children: make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-3", Summary: "Add SSO configuration admin panel",
			Type: "Story", TypeID: "2", Status: "To Do", Priority: "Medium",
			Assignee: "Jordan Park", Reporter: "Sarah Chen",
			Created: d(25), Updated: d(5), ParentKey: "DEMO-1",
			Components: "Admin, Auth",
			Desc:       md("Build an admin panel for configuring SSO providers (SAML, OIDC).\n\n## Requirements\n\n- Support multiple IdP configurations per tenant\n- Include connection testing (dry-run auth flow)\n- Provide clear error messages for misconfigured providers"),
			Comments: []jira.CommentView{
				{Author: "Jordan Park", Created: dt(5, 4),
					Body: md("I've started on the wireframes. Going with a tabbed layout — one tab per IdP type. Will share designs in Figma by EOD.")},
			},
			Children: make(map[string]*jira.IssueView),
		},
		// Sub-tasks under Story DEMO-2
		{
			Key: "DEMO-4", Summary: "Write unit tests for token exchange",
			Type: "Sub-task", TypeID: "5", Status: "In Progress", Priority: "High",
			Assignee: "Alex Rivera", Reporter: "Alex Rivera",
			Created: d(10), Updated: d(1), ParentKey: "DEMO-2",
			Desc:     md("Cover the token exchange endpoint with unit tests:\n\n- Happy path with valid PKCE\n- Missing code_verifier\n- Expired authorization code\n- Invalid redirect_uri mismatch"),
			Children: make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-5", Summary: "Handle refresh token rotation",
			Type: "Sub-task", TypeID: "5", Status: "To Do", Priority: "Medium",
			Assignee: "Unassigned", Reporter: "Alex Rivera",
			Created: d(8), Updated: d(8), ParentKey: "DEMO-2",
			Desc:     md("Implement refresh token rotation per [RFC 6749 Section 6](https://tools.ietf.org/html/rfc6749#section-6).\n\nEach refresh must issue a new refresh token and invalidate the old one."),
			Children: make(map[string]*jira.IssueView),
		},

		// ── Epic 2: Performance ────────────────────────────────────
		{
			Key: "DEMO-10", Summary: "API Performance Improvements",
			Type: "Epic", TypeID: "1", Status: "In Progress", Priority: "High",
			Assignee: "Mike Smith", Reporter: "Mike Smith",
			Created: d(60), Updated: d(3),
			Labels: "performance, sre", Components: "API",
			Desc: md("## Problem\n\nP95 latency on `/api/v2/search` has crept up to **1.2s**. Target is under 300ms.\n\n## Approach\n\n1. Add database query caching (Redis)\n2. Optimize N+1 queries in the search resolver\n3. Add response compression\n\n## Metrics\n\n| Endpoint | Current P95 | Target |\n|---|---|---|\n| /api/v2/search | 1200ms | 300ms |\n| /api/v2/issues | 450ms | 150ms |\n| /api/v2/users | 200ms | 100ms |"),
			Comments: []jira.CommentView{
				{Author: "Mike Smith", Created: dt(45, 6),
					Body: md("Setting up the epic. Priority is the search endpoint — it's impacting user retention.\n\n> P95 latency on search has crept up to 1.2s\n\nLet's get Redis caching in first (biggest bang for buck), then tackle the N+1 issue.")},
			},
			Children: make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-11", Summary: "Add Redis caching layer for search queries",
			Type: "Task", TypeID: "3", Status: "Done", Priority: "High",
			Assignee: "Priya Patel", Reporter: "Mike Smith",
			Created: d(40), Updated: d(7), ParentKey: "DEMO-10",
			Components: "API, Infrastructure",
			Desc:       md("Integrate Redis as a read-through cache for the search endpoint.\n\n## Implementation Notes\n\n```go\nfunc (s *SearchService) Search(ctx context.Context, q Query) ([]Result, error) {\n    key := q.CacheKey()\n    if cached, ok := s.cache.Get(ctx, key); ok {\n        return cached, nil\n    }\n    results, err := s.repo.Search(ctx, q)\n    if err != nil {\n        return nil, err\n    }\n    s.cache.Set(ctx, key, results, 5*time.Minute)\n    return results, nil\n}\n```\n\nCache invalidation happens on write via pub/sub."),
			Comments: []jira.CommentView{
				{Author: "Priya Patel", Created: dt(7, 3),
					Body: md("Deployed to staging. P95 dropped from 1200ms to **380ms** on the search endpoint. Still need to tune TTL and add cache warming.")},
			},
			Children: make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-12", Summary: "Fix N+1 queries in issue resolver",
			Type: "Task", TypeID: "3", Status: "In Progress", Priority: "High",
			Assignee: "Priya Patel", Reporter: "Mike Smith",
			Created: d(20), Updated: d(2), ParentKey: "DEMO-10",
			Components: "API",
			Desc:       md("The issue resolver makes a separate DB query for each issue's assignee and comments. Use `DataLoader` pattern to batch these.\n\nExpected improvement: ~200ms reduction on `/api/v2/issues`."),
			Comments: []jira.CommentView{
				{Author: "Priya Patel", Created: dt(2, 8),
					Body: md("Profiling shows the main bottleneck is in `resolveAssignees` — 47 queries for a page of 25 issues. DataLoader should reduce this to 2 batched queries.")},
				{Author: "Mike Smith", Created: dt(1, 5),
					Body: md("Nice find. Let's make sure we add a **max batch size** cap to prevent unbounded `IN` clauses. 100 should be safe for Postgres.")},
			},
			Children: make(map[string]*jira.IssueView),
		},

		// ── Bug ────────────────────────────────────────────────────
		{
			Key: "DEMO-20", Summary: "Login fails silently when IdP returns error",
			Type: "Bug", TypeID: "4", Status: "To Do", Priority: "Highest",
			Assignee: "Alex Rivera", Reporter: "Jordan Park",
			Created: d(2), Updated: d(1),
			Labels: "bug, security", Components: "Auth",
			Desc: md("## Steps to Reproduce\n\n1. Configure an IdP with an invalid client_secret\n2. Attempt to log in\n3. The callback fires but the error response from the IdP is swallowed\n\n## Expected\n\nUser sees a clear error: \"Authentication failed: Invalid client configuration\"\n\n## Actual\n\nUser is redirected back to the login page with no error message. The error is only visible in server logs."),
			Comments: []jira.CommentView{
				{Author: "Jordan Park", Created: dt(1, 6),
					Body: md("Found this during QA of the SSO work. It's a pre-existing issue but it'll get worse as we onboard more IdPs.")},
			},
			Children: make(map[string]*jira.IssueView),
		},

		// ── Standalone tasks ───────────────────────────────────────
		{
			Key: "DEMO-30", Summary: "Update API documentation for v2 endpoints",
			Type: "Task", TypeID: "3", Status: "Backlog", Priority: "Low",
			Assignee: "Unassigned", Reporter: "Sarah Chen",
			Created: d(15), Updated: d(15),
			Components: "Docs",
			Desc:       md("The API docs are out of date after the v2 migration. Key gaps:\n\n- Missing auth flow documentation\n- Rate limiting headers not documented\n- Webhook payload schemas need updating"),
			Children:   make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-31", Summary: "Set up CI pipeline for integration tests",
			Type: "Task", TypeID: "3", Status: "In Review", Priority: "Medium",
			Assignee: "Jordan Park", Reporter: "Mike Smith",
			Created: d(12), Updated: d(3),
			Labels: "devops", Components: "CI/CD",
			Desc:     md("Add a GitHub Actions workflow that runs integration tests against a Postgres + Redis test environment.\n\n## Requirements\n\n- Use `docker compose` for service dependencies\n- Run on every PR to `main`\n- Fail fast with clear error output\n- Cache Docker layers for speed"),
			Children: make(map[string]*jira.IssueView),
		},

		// ── Rich content demo issue (for scroll testing) ──────────
		{
			Key: "DEMO-50", Summary: "Comprehensive ADF rendering test",
			Type: "Story", TypeID: "2", Status: "In Progress", Priority: "Medium",
			Assignee: "Sarah Chen", Reporter: "Mike Smith",
			Created: d(5), Updated: d(1),
			Labels: "testing, documentation", Components: "UI, Core",
			Desc: md(`## Heading Level 1

### Heading Level 2

#### Heading Level 3

This is a standard paragraph. It contains multiple lines of text, but as long as there is no double line break, it remains a single paragraph block in Jira ADF. It also contains **bold text**, *italic text*, ~~strikethrough text~~, and some ` + "`inline code`" + `.

Here is a [link to Atlassian](https://atlassian.com) embedded right in the middle of a paragraph. You can also mix **[bold links](https://example.com)**.

**Unordered List Example:**

- First item in the bullet list
- Second item containing ` + "`inline code`" + `
- Third item containing a [link](https://example.com)

**Ordered List Example:**

1. First step in the process
2. Second step, which is **very important**
3. Third step

Here is a code block:

` + "```python\ndef hello_jira():\n    print(\"This should not be split into multiple paragraphs!\")\n    return True\n```" + `

Final paragraph after the code block.

---

## Another Section

> This is a blockquote. It should be rendered with a vertical bar on the left side and slightly indented text.

And here is a table:

| Feature | Status | Owner |
|---------|--------|-------|
| OAuth PKCE | Done | Alex |
| SSO Admin | In Progress | Jordan |
| Refresh Tokens | To Do | Unassigned |

### Nested Lists

- Top level item
  - Nested item A
  - Nested item B
    - Deeply nested item
- Another top level item

That wraps up the comprehensive test content. The preview should require scrolling to see all of this.`),
			Comments: []jira.CommentView{
				{Author: "Mike Smith", Created: dt(4, 10),
					Body: md("Created this issue to verify all the markdown rendering paths. Please check:\n\n1. Bold, italic, strikethrough\n2. Code blocks with syntax highlighting\n3. Tables\n4. Nested lists\n5. Links and blockquotes")},
				{Author: "Sarah Chen", Created: dt(3, 6),
					Body: md("Tables are rendering but column alignment could be better. The `code blocks` look good though.\n\n> I really like how the blockquotes turned out\n\n_Tested in iTerm2 and Alacritty — both look fine._")},
				{Author: "Alex Rivera", Created: dt(2, 4),
					Body: md("Found an edge case: **bold text that spans\nmultiple lines** doesn't render correctly. We should fix this in the renderer.")},
				{Author: "Jordan Park", Created: dt(1, 2),
					Body: md("LGTM on the rendering. One thing — the ~~strikethrough~~ style is very subtle on dark backgrounds. Can we make it more visible?\n\nAlso, links should probably use OSC 8 hyperlinks so they're clickable in supported terminals.")},
			},
			Children: make(map[string]*jira.IssueView),
		},

		// Children of the rich content issue (to test child rendering in preview)
		{
			Key: "DEMO-51", Summary: "Fix bold text across line breaks",
			Type: "Sub-task", TypeID: "5", Status: "To Do", Priority: "Low",
			Assignee: "Alex Rivera", Reporter: "Sarah Chen",
			Created: d(2), Updated: d(2), ParentKey: "DEMO-50",
			Desc:     md("When bold text spans a line break in the source markdown, the renderer should keep the entire span bold.\n\nCurrent behavior: bold is reset at the newline."),
			Children: make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-52", Summary: "Improve strikethrough visibility on dark themes",
			Type: "Sub-task", TypeID: "5", Status: "In Progress", Priority: "Low",
			Assignee: "Jordan Park", Reporter: "Sarah Chen",
			Created: d(2), Updated: d(1), ParentKey: "DEMO-50",
			Desc:     md("Strikethrough (~~text~~) is hard to see on dark terminal backgrounds. Consider using dim + strikethrough together, or a different color."),
			Children: make(map[string]*jira.IssueView),
		},
		{
			Key: "DEMO-53", Summary: "Add OSC 8 hyperlink support",
			Type: "Sub-task", TypeID: "5", Status: "Done", Priority: "Medium",
			Assignee: "Sarah Chen", Reporter: "Mike Smith",
			Created: d(4), Updated: d(1), ParentKey: "DEMO-50",
			Desc:     md("Use OSC 8 escape sequences for clickable links in terminals that support them (iTerm2, Alacritty, etc.).\n\nFallback: show `text (url)` for unsupported terminals."),
			Children: make(map[string]*jira.IssueView),
		},
	}
}
