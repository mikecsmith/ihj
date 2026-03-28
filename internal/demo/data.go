package demo

import (
	"time"

	"github.com/mikecsmith/ihj/internal/core"
	"github.com/mikecsmith/ihj/internal/document"
)

// Workspace returns the synthetic workspace for demo mode.
func Workspace() *core.Workspace {
	types := []core.TypeConfig{
		{ID: 1, Name: "Epic", Order: 20, Color: "magenta", HasChildren: true},
		{ID: 2, Name: "Story", Order: 30, Color: "cyan", HasChildren: true},
		{ID: 3, Name: "Task", Order: 30, Color: "default", HasChildren: true},
		{ID: 4, Name: "Bug", Order: 30, Color: "red", HasChildren: false},
		{ID: 5, Name: "Sub-task", Order: 40, Color: "white", HasChildren: false},
	}

	typeOrderMap := make(map[string]core.TypeOrderEntry, len(types))
	for _, t := range types {
		typeOrderMap[t.Name] = core.TypeOrderEntry{
			Order: t.Order, Color: t.Color, HasChildren: t.HasChildren,
		}
	}

	statuses := []string{"Backlog", "To Do", "In Progress", "In Review", "Done"}
	statusWeights := make(map[string]int, len(statuses))
	for i, s := range statuses {
		statusWeights[s] = i
	}

	return &core.Workspace{
		Slug:          "demo",
		Name:          "Demo Board",
		Provider:      core.ProviderDemo,
		Types:         types,
		Statuses:      statuses,
		StatusWeights: statusWeights,
		TypeOrderMap:  typeOrderMap,
		Filters:       map[string]string{"active": ""},
	}
}

// Issues returns the synthetic work items for demo mode.
func Issues() []*core.WorkItem {
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

	return []*core.WorkItem{
		// ── Epic 1: Authentication ──────────────────────────────────
		{
			ID: "DEMO-1", Summary: "User Authentication Overhaul",
			Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "Sarah Chen",
				"reporter":   "Mike Smith",
				"created":    d(45),
				"updated":    d(2),
				"labels":     []string{"security", "q1-priority"},
				"components": []string{"Auth"},
			},
			Description: md("## Overview\n\nReplace the legacy session-based authentication with **OAuth 2.0 + PKCE** flow.\n\n## Goals\n\n- Eliminate session token storage compliance issues\n- Support SSO via SAML/OIDC\n- Reduce login friction by 40%\n\n## Out of Scope\n\n- Migration of existing user sessions (handled by DEMO-15)\n- Mobile app auth (separate epic)"),
			Comments: []core.Comment{
				{Author: "Mike Smith", Created: dt(30, 3),
					Body: md("Kicked off the epic. Sarah is leading this — let's aim to have the PKCE flow in staging by end of sprint 4.")},
				{Author: "Sarah Chen", Created: dt(14, 2),
					Body: md("Quick update: PKCE implementation is **on track**. The admin panel (DEMO-3) might slip to next sprint due to design dependency.\n\n> The IdP error handling (DEMO-20) is a pre-existing issue we should fix in parallel.\n\n_— Sarah_")},
			},
		},
		{
			ID: "DEMO-2", Summary: "Implement OAuth 2.0 PKCE login flow",
			Type: "Story", Status: "In Review", ParentID: "DEMO-1",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "Alex Rivera",
				"reporter":   "Sarah Chen",
				"created":    d(30),
				"updated":    d(1),
				"labels":     []string{"security"},
				"components": []string{"Auth", "API"},
			},
			Description: md("Implement the full OAuth 2.0 Authorization Code flow with PKCE.\n\n## Acceptance Criteria\n\n- Given a user visits the login page, when they click Sign In, then they are redirected to the IdP\n- Given the IdP returns an auth code, when the callback fires, then a valid access token is issued\n- Given the PKCE verifier is missing, when the token exchange occurs, then the request is rejected with 400\n\n## Technical Notes\n\nUse `S256` for the code challenge method. Plain is deprecated per [RFC 7636](https://tools.ietf.org/html/rfc7636).\n\n```go\nfunc generatePKCE() (verifier, challenge string) {\n    b := make([]byte, 32)\n    rand.Read(b)\n    verifier = base64.RawURLEncoding.EncodeToString(b)\n    h := sha256.Sum256([]byte(verifier))\n    challenge = base64.RawURLEncoding.EncodeToString(h[:])\n    return\n}\n```"),
			Comments: []core.Comment{
				{Author: "Alex Rivera", Created: dt(10, 5),
					Body: md("Started the implementation. The PKCE challenge generation is working. Still need to wire up the callback handler.")},
				{Author: "Sarah Chen", Created: dt(3, 8),
					Body: md("Looks great so far. Can we make sure we're using `S256` for the code challenge method? Plain is deprecated.")},
				{Author: "Alex Rivera", Created: dt(1, 2),
					Body: md("Done — PR is up for review. Key changes:\n\n1. Added PKCE challenge/verifier generation\n2. Updated the `/auth/callback` handler\n3. Added integration tests against the mock IdP\n\nThe **token exchange** is fully tested. @Sarah can you review when you get a chance?")},
			},
		},
		{
			ID: "DEMO-3", Summary: "Add SSO configuration admin panel",
			Type: "Story", Status: "To Do", ParentID: "DEMO-1",
			Fields: map[string]any{
				"priority":   "Medium",
				"assignee":   "Jordan Park",
				"reporter":   "Sarah Chen",
				"created":    d(25),
				"updated":    d(5),
				"components": []string{"Admin", "Auth"},
			},
			Description: md("Build an admin panel for configuring SSO providers (SAML, OIDC).\n\n## Requirements\n\n- Support multiple IdP configurations per tenant\n- Include connection testing (dry-run auth flow)\n- Provide clear error messages for misconfigured providers"),
			Comments: []core.Comment{
				{Author: "Jordan Park", Created: dt(5, 4),
					Body: md("I've started on the wireframes. Going with a tabbed layout — one tab per IdP type. Will share designs in Figma by EOD.")},
			},
		},
		{
			ID: "DEMO-4", Summary: "Write unit tests for token exchange",
			Type: "Sub-task", Status: "In Progress", ParentID: "DEMO-2",
			Fields: map[string]any{
				"priority": "High",
				"assignee": "Alex Rivera",
				"reporter": "Alex Rivera",
				"created":  d(10),
				"updated":  d(1),
			},
			Description: md("Cover the token exchange endpoint with unit tests:\n\n- Happy path with valid PKCE\n- Missing code_verifier\n- Expired authorization code\n- Invalid redirect_uri mismatch"),
		},
		{
			ID: "DEMO-5", Summary: "Handle refresh token rotation",
			Type: "Sub-task", Status: "To Do", ParentID: "DEMO-2",
			Fields: map[string]any{
				"priority": "Medium",
				"assignee": "Unassigned",
				"reporter": "Alex Rivera",
				"created":  d(8),
				"updated":  d(8),
			},
			Description: md("Implement refresh token rotation per [RFC 6749 Section 6](https://tools.ietf.org/html/rfc6749#section-6).\n\nEach refresh must issue a new refresh token and invalidate the old one."),
		},
		{
			ID: "DEMO-10", Summary: "API Performance Improvements",
			Type: "Epic", Status: "In Progress",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "Mike Smith",
				"reporter":   "Mike Smith",
				"created":    d(60),
				"updated":    d(3),
				"labels":     []string{"performance", "sre"},
				"components": []string{"API"},
			},
			Description: md("## Problem\n\nP95 latency on `/api/v2/search` has crept up to **1.2s**. Target is under 300ms.\n\n## Approach\n\n1. Add database query caching (Redis)\n2. Optimize N+1 queries in the search resolver\n3. Add response compression\n\n## Metrics\n\n| Endpoint | Current P95 | Target |\n|---|---|---|\n| /api/v2/search | 1200ms | 300ms |\n| /api/v2/issues | 450ms | 150ms |\n| /api/v2/users | 200ms | 100ms |"),
			Comments: []core.Comment{
				{Author: "Mike Smith", Created: dt(45, 6),
					Body: md("Setting up the epic. Priority is the search endpoint — it's impacting user retention.\n\n> P95 latency on search has crept up to 1.2s\n\nLet's get Redis caching in first (biggest bang for buck), then tackle the N+1 issue.")},
			},
		},
		{
			ID: "DEMO-11", Summary: "Add Redis caching layer for search queries",
			Type: "Task", Status: "Done", ParentID: "DEMO-10",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "Priya Patel",
				"reporter":   "Mike Smith",
				"created":    d(40),
				"updated":    d(7),
				"components": []string{"API", "Infrastructure"},
			},
			Description: md("Integrate Redis as a read-through cache for the search endpoint.\n\n## Implementation Notes\n\n```go\nfunc (s *SearchService) Search(ctx context.Context, q Query) ([]Result, error) {\n    key := q.CacheKey()\n    if cached, ok := s.cache.Get(ctx, key); ok {\n        return cached, nil\n    }\n    results, err := s.repo.Search(ctx, q)\n    if err != nil {\n        return nil, err\n    }\n    s.cache.Set(ctx, key, results, 5*time.Minute)\n    return results, nil\n}\n```\n\nCache invalidation happens on write via pub/sub."),
			Comments: []core.Comment{
				{Author: "Priya Patel", Created: dt(7, 3),
					Body: md("Deployed to staging. P95 dropped from 1200ms to **380ms** on the search endpoint. Still need to tune TTL and add cache warming.")},
			},
		},
		{
			ID: "DEMO-12", Summary: "Fix N+1 queries in issue resolver",
			Type: "Task", Status: "In Progress", ParentID: "DEMO-10",
			Fields: map[string]any{
				"priority":   "High",
				"assignee":   "Priya Patel",
				"reporter":   "Mike Smith",
				"created":    d(20),
				"updated":    d(2),
				"components": []string{"API"},
			},
			Description: md("The issue resolver makes a separate DB query for each issue's assignee and comments. Use `DataLoader` pattern to batch these.\n\nExpected improvement: ~200ms reduction on `/api/v2/issues`."),
			Comments: []core.Comment{
				{Author: "Priya Patel", Created: dt(2, 8),
					Body: md("Profiling shows the main bottleneck is in `resolveAssignees` — 47 queries for a page of 25 issues. DataLoader should reduce this to 2 batched queries.")},
				{Author: "Mike Smith", Created: dt(1, 5),
					Body: md("Nice find. Let's make sure we add a **max batch size** cap to prevent unbounded `IN` clauses. 100 should be safe for Postgres.")},
			},
		},
		{
			ID: "DEMO-20", Summary: "Login fails silently when IdP returns error",
			Type: "Bug", Status: "To Do",
			Fields: map[string]any{
				"priority":   "Highest",
				"assignee":   "Alex Rivera",
				"reporter":   "Jordan Park",
				"created":    d(2),
				"updated":    d(1),
				"labels":     []string{"bug", "security"},
				"components": []string{"Auth"},
			},
			Description: md("## Steps to Reproduce\n\n1. Configure an IdP with an invalid client_secret\n2. Attempt to log in\n3. The callback fires but the error response from the IdP is swallowed\n\n## Expected\n\nUser sees a clear error: \"Authentication failed: Invalid client configuration\"\n\n## Actual\n\nUser is redirected back to the login page with no error message. The error is only visible in server logs."),
			Comments: []core.Comment{
				{Author: "Jordan Park", Created: dt(1, 6),
					Body: md("Found this during QA of the SSO work. It's a pre-existing issue but it'll get worse as we onboard more IdPs.")},
			},
		},
		{
			ID: "DEMO-30", Summary: "Update API documentation for v2 endpoints",
			Type: "Task", Status: "Backlog",
			Fields: map[string]any{
				"priority":   "Low",
				"assignee":   "Unassigned",
				"reporter":   "Sarah Chen",
				"created":    d(15),
				"updated":    d(15),
				"components": []string{"Docs"},
			},
			Description: md("The API docs are out of date after the v2 migration. Key gaps:\n\n- Missing auth flow documentation\n- Rate limiting headers not documented\n- Webhook payload schemas need updating"),
		},
		{
			ID: "DEMO-31", Summary: "Set up CI pipeline for integration tests",
			Type: "Task", Status: "In Review",
			Fields: map[string]any{
				"priority":   "Medium",
				"assignee":   "Jordan Park",
				"reporter":   "Mike Smith",
				"created":    d(12),
				"updated":    d(3),
				"labels":     []string{"devops"},
				"components": []string{"CI/CD"},
			},
			Description: md("Add a GitHub Actions workflow that runs integration tests against a Postgres + Redis test environment.\n\n## Requirements\n\n- Use `docker compose` for service dependencies\n- Run on every PR to `main`\n- Fail fast with clear error output\n- Cache Docker layers for speed"),
		},
		{
			ID: "DEMO-50", Summary: "Comprehensive ADF rendering test",
			Type: "Story", Status: "In Progress",
			Fields: map[string]any{
				"priority":   "Medium",
				"assignee":   "Sarah Chen",
				"reporter":   "Mike Smith",
				"created":    d(5),
				"updated":    d(1),
				"labels":     []string{"testing", "documentation"},
				"components": []string{"UI", "Core"},
			},
			Description: md(`## Heading Level 1

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
			Comments: []core.Comment{
				{Author: "Mike Smith", Created: dt(4, 10),
					Body: md("Created this issue to verify all the markdown rendering paths. Please check:\n\n1. Bold, italic, strikethrough\n2. Code blocks with syntax highlighting\n3. Tables\n4. Nested lists\n5. Links and blockquotes")},
				{Author: "Sarah Chen", Created: dt(3, 6),
					Body: md("Tables are rendering but column alignment could be better. The `code blocks` look good though.\n\n> I really like how the blockquotes turned out\n\n_Tested in iTerm2 and Alacritty — both look fine._")},
				{Author: "Alex Rivera", Created: dt(2, 4),
					Body: md("Found an edge case: **bold text that spans\nmultiple lines** doesn't render correctly. We should fix this in the renderer.")},
				{Author: "Jordan Park", Created: dt(1, 2),
					Body: md("LGTM on the rendering. One thing — the ~~strikethrough~~ style is very subtle on dark backgrounds. Can we make it more visible?\n\nAlso, links should probably use OSC 8 hyperlinks so they're clickable in supported terminals.")},
			},
		},
		{
			ID: "DEMO-51", Summary: "Fix bold text across line breaks",
			Type: "Sub-task", Status: "To Do", ParentID: "DEMO-50",
			Fields: map[string]any{
				"priority": "Low",
				"assignee": "Alex Rivera",
				"reporter": "Sarah Chen",
				"created":  d(2),
				"updated":  d(2),
			},
			Description: md("When bold text spans a line break in the source markdown, the renderer should keep the entire span bold.\n\nCurrent behavior: bold is reset at the newline."),
		},
		{
			ID: "DEMO-52", Summary: "Improve strikethrough visibility on dark themes",
			Type: "Sub-task", Status: "In Progress", ParentID: "DEMO-50",
			Fields: map[string]any{
				"priority": "Low",
				"assignee": "Jordan Park",
				"reporter": "Sarah Chen",
				"created":  d(2),
				"updated":  d(1),
			},
			Description: md("Strikethrough (~~text~~) is hard to see on dark terminal backgrounds. Consider using dim + strikethrough together, or a different color."),
		},
		{
			ID: "DEMO-53", Summary: "Add OSC 8 hyperlink support",
			Type: "Sub-task", Status: "Done", ParentID: "DEMO-50",
			Fields: map[string]any{
				"priority": "Medium",
				"assignee": "Sarah Chen",
				"reporter": "Mike Smith",
				"created":  d(4),
				"updated":  d(1),
			},
			Description: md("Use OSC 8 escape sequences for clickable links in terminals that support them (iTerm2, Alacritty, etc.).\n\nFallback: show `text (url)` for unsupported terminals."),
		},
	}
}
