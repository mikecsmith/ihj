package fakejira

import (
	"time"
)

// adfDoc wraps some plain text in a minimal Atlassian Document Format
// envelope so the Jira provider can round-trip it through its ADF
// renderer the same way production issues do.
func adfDoc(paragraphs ...string) map[string]any {
	content := make([]any, 0, len(paragraphs))
	for _, p := range paragraphs {
		content = append(content, map[string]any{
			"type": "paragraph",
			"content": []any{
				map[string]any{"type": "text", "text": p},
			},
		})
	}
	return map[string]any{"type": "doc", "version": 1, "content": content}
}

// standardUsers seeds a common cast of demo users. Every seed function
// calls this so both the scrum and kanban workspaces share the same
// people and `assignee = currentUser()` resolves consistently.
func standardUsers(s *State) {
	s.users["demo-user"] = &entUser{AccountID: "demo-user", DisplayName: "Demo User", Email: "demo@example.com"}
	s.users["sarah"] = &entUser{AccountID: "sarah", DisplayName: "Sarah Chen", Email: "sarah@example.com"}
	s.users["alex"] = &entUser{AccountID: "alex", DisplayName: "Alex Rivera", Email: "alex@example.com"}
	s.users["jordan"] = &entUser{AccountID: "jordan", DisplayName: "Jordan Patel", Email: "jordan@example.com"}
	s.users["morgan"] = &entUser{AccountID: "morgan", DisplayName: "Morgan Lee", Email: "morgan@example.com"}
	s.users["riley"] = &entUser{AccountID: "riley", DisplayName: "Riley Nakamura", Email: "riley@example.com"}
	s.Me = "demo-user"
}

// standardPriorities seeds Jira's default priority ladder.
func standardPriorities(s *State) {
	s.priorities = []*entPriority{
		{ID: "1", Name: "Highest"},
		{ID: "2", Name: "High"},
		{ID: "3", Name: "Medium"},
		{ID: "4", Name: "Low"},
		{ID: "5", Name: "Lowest"},
	}
}

// Seed populates the state with the scrum demo scenario: a single DEMO
// project with an epic tree, bugs, chores, two active sprints, a future
// sprint, and a deep backlog so the TUI's filters, sprint boundaries,
// and hierarchical tree view all have data to exercise.
func Seed(s *State) {
	standardUsers(s)
	standardPriorities(s)

	s.statuses = []*entStatus{
		{ID: "1", Name: "To Do", CategoryKey: "new"},
		{ID: "2", Name: "In Progress", CategoryKey: "indeterminate"},
		{ID: "3", Name: "In Review", CategoryKey: "indeterminate"},
		{ID: "4", Name: "Done", CategoryKey: "done"},
	}

	s.types = []*entIssueType{
		{ID: "10", Name: "Epic", Subtask: false},
		{ID: "11", Name: "Story", Subtask: false},
		{ID: "12", Name: "Task", Subtask: false},
		{ID: "13", Name: "Bug", Subtask: false},
		{ID: "14", Name: "Sub-task", Subtask: true},
	}

	s.sprints[101] = &entSprint{ID: 101, Name: "DEMO Sprint 12", State: "active", BoardID: BoardID}
	s.sprints[102] = &entSprint{ID: 102, Name: "DEMO Sprint 13", State: "future", BoardID: BoardID}

	now := time.Now().UTC()
	daysAgo := func(d int) time.Time { return now.AddDate(0, 0, -d) }

	// ── Epic 1: Authentication overhaul ───────────────────────────────────
	epicAuth := s.CreateIssue(func(i *entIssue) {
		i.Summary = "User Authentication Overhaul"
		i.TypeID = "10"
		i.StatusID = "2"
		i.PriorityID = "2"
		i.AssigneeID = "sarah"
		i.ReporterID = "demo-user"
		i.Labels = []string{"security", "q1-priority"}
		i.Components = []string{"Auth"}
		i.Created = daysAgo(45)
		i.Updated = daysAgo(2)
		i.SprintID = 101
		i.Description = adfDoc(
			"Replace legacy session auth with OAuth 2.0 + PKCE.",
			"Support SSO via SAML/OIDC for enterprise tenants.",
		)
		i.Customs = map[string]any{CFStoryPoints: 21}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Implement OAuth 2.0 PKCE login flow"
		i.TypeID = "11"
		i.StatusID = "3"
		i.PriorityID = "2"
		i.AssigneeID = "alex"
		i.ReporterID = "sarah"
		i.ParentKey = epicAuth.Key
		i.Labels = []string{"backend"}
		i.Created = daysAgo(30)
		i.Updated = daysAgo(1)
		i.SprintID = 101
		i.Description = adfDoc("Build the authorization-code + PKCE login flow, including the redirect handler.")
		i.Customs = map[string]any{CFStoryPoints: 8}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Admin panel: rotate client secrets"
		i.TypeID = "11"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "jordan"
		i.ReporterID = "sarah"
		i.ParentKey = epicAuth.Key
		i.Created = daysAgo(20)
		i.Updated = daysAgo(5)
		i.SprintID = 102
		i.Description = adfDoc("Let tenant admins rotate OAuth client secrets from the settings page.")
		i.Customs = map[string]any{CFStoryPoints: 5}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Migrate legacy sessions on first login"
		i.TypeID = "12"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "demo-user"
		i.ReporterID = "sarah"
		i.ParentKey = epicAuth.Key
		i.Created = daysAgo(14)
		i.Updated = daysAgo(14)
		i.SprintID = 102
		i.Description = adfDoc("Transparently upgrade existing cookie sessions when the user next logs in.")
		i.Customs = map[string]any{CFStoryPoints: 3}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "SAML assertion parser"
		i.TypeID = "11"
		i.StatusID = "1"
		i.PriorityID = "2"
		i.AssigneeID = "morgan"
		i.ReporterID = "sarah"
		i.ParentKey = epicAuth.Key
		i.Labels = []string{"backend", "sso"}
		i.Created = daysAgo(12)
		i.Updated = daysAgo(8)
		i.Description = adfDoc("Parse and validate SAML 2.0 assertions for enterprise SSO.")
		i.Customs = map[string]any{CFStoryPoints: 13}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "OIDC discovery endpoint cache"
		i.TypeID = "12"
		i.StatusID = "1"
		i.PriorityID = "4"
		i.AssigneeID = "alex"
		i.ReporterID = "morgan"
		i.ParentKey = epicAuth.Key
		i.Created = daysAgo(9)
		i.Updated = daysAgo(9)
		i.Description = adfDoc("Cache OIDC provider metadata with sensible TTLs; refresh on demand.")
		i.Customs = map[string]any{CFStoryPoints: 3}
	})

	// ── Epic 2: Billing rework ────────────────────────────────────────────
	epicBill := s.CreateIssue(func(i *entIssue) {
		i.Summary = "Billing: usage-based pricing"
		i.TypeID = "10"
		i.StatusID = "2"
		i.PriorityID = "2"
		i.AssigneeID = "riley"
		i.ReporterID = "demo-user"
		i.Labels = []string{"billing", "q1-priority"}
		i.Components = []string{"Billing"}
		i.Created = daysAgo(60)
		i.Updated = daysAgo(3)
		i.SprintID = 101
		i.Description = adfDoc(
			"Introduce usage-based billing alongside flat-rate plans.",
			"Requires metering, invoice line items, and a dunning flow.",
		)
		i.Customs = map[string]any{CFStoryPoints: 34}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Meter API requests per workspace"
		i.TypeID = "11"
		i.StatusID = "2"
		i.PriorityID = "2"
		i.AssigneeID = "riley"
		i.ReporterID = "demo-user"
		i.ParentKey = epicBill.Key
		i.Labels = []string{"backend", "metering"}
		i.Created = daysAgo(40)
		i.Updated = daysAgo(1)
		i.SprintID = 101
		i.Description = adfDoc("Count API requests per workspace and flush to the metering pipeline every 60s.")
		i.Customs = map[string]any{CFStoryPoints: 8}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Invoice line items from usage events"
		i.TypeID = "11"
		i.StatusID = "1"
		i.PriorityID = "2"
		i.AssigneeID = "riley"
		i.ReporterID = "demo-user"
		i.ParentKey = epicBill.Key
		i.Created = daysAgo(25)
		i.Updated = daysAgo(4)
		i.SprintID = 101
		i.Description = adfDoc("Roll metered usage up into monthly invoice line items keyed by SKU.")
		i.Customs = map[string]any{CFStoryPoints: 8}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Dunning emails for past-due invoices"
		i.TypeID = "12"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "morgan"
		i.ReporterID = "riley"
		i.ParentKey = epicBill.Key
		i.Labels = []string{"email"}
		i.Created = daysAgo(18)
		i.Updated = daysAgo(18)
		i.SprintID = 102
		i.Description = adfDoc("Send escalating reminders at 7/14/30 days past due.")
		i.Customs = map[string]any{CFStoryPoints: 5}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Tax calculation for EU VAT"
		i.TypeID = "11"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "jordan"
		i.ReporterID = "riley"
		i.ParentKey = epicBill.Key
		i.Labels = []string{"tax", "eu"}
		i.Created = daysAgo(11)
		i.Updated = daysAgo(11)
		i.Description = adfDoc("Compute VAT by member state with reverse-charge for B2B customers.")
		i.Customs = map[string]any{CFStoryPoints: 13}
	})

	// ── Epic 3: Developer experience ──────────────────────────────────────
	epicDX := s.CreateIssue(func(i *entIssue) {
		i.Summary = "Developer experience: v2 SDK"
		i.TypeID = "10"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "morgan"
		i.ReporterID = "demo-user"
		i.Labels = []string{"sdk", "q2"}
		i.Components = []string{"SDK"}
		i.Created = daysAgo(20)
		i.Updated = daysAgo(6)
		i.Description = adfDoc("Ship v2 SDKs for Go, TypeScript, and Python with typed request/response models.")
		i.Customs = map[string]any{CFStoryPoints: 21}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Code-generate TypeScript client from OpenAPI"
		i.TypeID = "11"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "morgan"
		i.ReporterID = "demo-user"
		i.ParentKey = epicDX.Key
		i.Created = daysAgo(15)
		i.Updated = daysAgo(6)
		i.Description = adfDoc("Drive SDK codegen from the OpenAPI spec so clients stay in lockstep with the API.")
		i.Customs = map[string]any{CFStoryPoints: 8}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Go SDK: retry + rate-limit helpers"
		i.TypeID = "12"
		i.StatusID = "1"
		i.PriorityID = "4"
		i.AssigneeID = "demo-user"
		i.ReporterID = "morgan"
		i.ParentKey = epicDX.Key
		i.Created = daysAgo(10)
		i.Updated = daysAgo(10)
		i.Description = adfDoc("Built-in exponential backoff and bucket-aware rate-limit handling for the Go client.")
		i.Customs = map[string]any{CFStoryPoints: 5}
	})

	// ── Independent bugs and tasks ────────────────────────────────────────
	s.CreateIssue(func(i *entIssue) {
		i.Summary = "IdP returns 500 on missing email claim"
		i.TypeID = "13"
		i.StatusID = "2"
		i.PriorityID = "1"
		i.AssigneeID = "demo-user"
		i.ReporterID = "jordan"
		i.Labels = []string{"bug", "auth"}
		i.Created = daysAgo(5)
		i.Updated = daysAgo(1)
		i.SprintID = 101
		i.Description = adfDoc("When the IdP response omits the email claim, the login endpoint 500s instead of returning a friendly error.")
		i.Customs = map[string]any{CFStoryPoints: 2}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Browser back-button from callback loops the login page"
		i.TypeID = "13"
		i.StatusID = "4"
		i.PriorityID = "3"
		i.AssigneeID = "jordan"
		i.ReporterID = "demo-user"
		i.Created = daysAgo(21)
		i.Updated = daysAgo(7)
		i.SprintID = 101
		i.Description = adfDoc("Hitting back from /auth/callback sends the user back through the OAuth flow again.")
		i.Customs = map[string]any{CFStoryPoints: 1}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Off-by-one in pagination cursor decoder"
		i.TypeID = "13"
		i.StatusID = "3"
		i.PriorityID = "2"
		i.AssigneeID = "alex"
		i.ReporterID = "riley"
		i.Labels = []string{"bug", "api"}
		i.Created = daysAgo(3)
		i.Updated = daysAgo(1)
		i.SprintID = 101
		i.Description = adfDoc("Cursor pagination drops the last item of each page under specific filters.")
		i.Customs = map[string]any{CFStoryPoints: 2}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Document the new auth architecture"
		i.TypeID = "12"
		i.StatusID = "1"
		i.PriorityID = "4"
		i.AssigneeID = "demo-user"
		i.ReporterID = "sarah"
		i.Labels = []string{"docs"}
		i.Created = daysAgo(10)
		i.Updated = daysAgo(10)
		i.Description = adfDoc("Write the public docs page explaining the new login flow.")
		i.Customs = map[string]any{CFStoryPoints: 2}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Upgrade Postgres driver to v5"
		i.TypeID = "12"
		i.StatusID = "4"
		i.PriorityID = "4"
		i.AssigneeID = "alex"
		i.ReporterID = "alex"
		i.Labels = []string{"chore", "deps"}
		i.Created = daysAgo(28)
		i.Updated = daysAgo(14)
		i.Description = adfDoc("Move from pgx v4 to v5; audit connection pool settings.")
		i.Customs = map[string]any{CFStoryPoints: 3}
	})

	s.CreateIssue(func(i *entIssue) {
		i.Summary = "Audit log retention policy"
		i.TypeID = "12"
		i.StatusID = "1"
		i.PriorityID = "3"
		i.AssigneeID = "sarah"
		i.ReporterID = "demo-user"
		i.Labels = []string{"compliance"}
		i.Created = daysAgo(6)
		i.Updated = daysAgo(6)
		i.Description = adfDoc("Agree on per-tenant retention windows and land the pruning job.")
		i.Customs = map[string]any{CFStoryPoints: 3}
	})

	// Comments on a couple of issues for richer detail views.
	_ = s.AppendComment(epicAuth.Key, "sarah", adfDoc("Kicked off — PKCE flow is on track for sprint end."))
	_ = s.AppendComment(epicAuth.Key, "alex", adfDoc("Redirect handler landed behind a feature flag. Tests green."))
	_ = s.AppendComment(epicBill.Key, "riley", adfDoc("Metering pipeline is the critical path; aligning with infra."))
}

// SeedKanban populates a kanban-style OPS project — no sprints, a
// dedicated support workflow (Triage → In Progress → Blocked → Done),
// and a flat issue list. Used by the second demo workspace.
func SeedKanban(s *State) {
	standardUsers(s)
	standardPriorities(s)

	s.statuses = []*entStatus{
		{ID: "1", Name: "Triage", CategoryKey: "new"},
		{ID: "2", Name: "In Progress", CategoryKey: "indeterminate"},
		{ID: "3", Name: "Blocked", CategoryKey: "indeterminate"},
		{ID: "4", Name: "Done", CategoryKey: "done"},
	}

	s.types = []*entIssueType{
		{ID: "10", Name: "Incident", Subtask: false},
		{ID: "11", Name: "Request", Subtask: false},
		{ID: "12", Name: "Task", Subtask: false},
		{ID: "13", Name: "Bug", Subtask: false},
	}

	now := time.Now().UTC()
	daysAgo := func(d int) time.Time { return now.AddDate(0, 0, -d) }
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }

	cases := []struct {
		summary     string
		typeID      string
		statusID    string
		priorityID  string
		assignee    string
		reporter    string
		labels      []string
		components  []string
		created     time.Time
		updated     time.Time
		description string
		storyPts    int
	}{
		{
			"Customer API returning 503s intermittently", "10", "2", "1",
			"demo-user", "riley", []string{"incident", "sev2"}, []string{"API"},
			hoursAgo(6), hoursAgo(1),
			"Sporadic 503s from the east-1 region. Investigation ongoing.", 5,
		},
		{
			"Email digest missing unread counts for large workspaces", "13", "1", "2",
			"morgan", "sarah", []string{"bug", "email"}, []string{"Notifications"},
			daysAgo(1), daysAgo(1),
			"Digest shows 0 unread when the inbox has >1k items.", 3,
		},
		{
			"Increase rate-limit ceiling for acme-corp", "11", "3", "2",
			"alex", "demo-user", []string{"customer"}, []string{"API"},
			daysAgo(2), hoursAgo(18),
			"Waiting on SecOps signoff before lifting the cap.", 1,
		},
		{
			"Migrate logging from ELK to Loki", "12", "2", "3",
			"morgan", "morgan", []string{"infra"}, []string{"Platform"},
			daysAgo(12), daysAgo(3),
			"Cost reduction initiative; run parallel for 2 weeks.", 8,
		},
		{
			"Onboard new support engineer", "12", "1", "4",
			"sarah", "demo-user", []string{"onboarding"}, nil,
			daysAgo(3), daysAgo(3),
			"Jordan joins on Monday — pair schedule, accounts, runbook walk-through.", 2,
		},
		{
			"PagerDuty routing wrong on weekends", "13", "2", "2",
			"demo-user", "jordan", []string{"bug", "oncall"}, []string{"Oncall"},
			daysAgo(4), hoursAgo(4),
			"Weekend escalations currently page the weekday primary.", 2,
		},
		{
			"Add runbook for DB failover", "12", "1", "3",
			"alex", "sarah", []string{"docs", "runbook"}, []string{"Platform"},
			daysAgo(5), daysAgo(5),
			"Document the manual failover steps for the primary Postgres cluster.", 3,
		},
		{
			"Spike: evaluate Grafana Cloud", "12", "4", "4",
			"riley", "morgan", []string{"spike"}, nil,
			daysAgo(20), daysAgo(8),
			"Compare Grafana Cloud vs. self-hosted — findings in Confluence.", 5,
		},
		{
			"Stripe webhook retries exhausted for customer 4481", "10", "3", "1",
			"riley", "demo-user", []string{"incident", "billing"}, []string{"Billing"},
			hoursAgo(36), hoursAgo(12),
			"Waiting on Stripe support to re-emit the missing events.", 3,
		},
		{
			"Add '/healthz' endpoint to auth service", "12", "4", "3",
			"alex", "alex", []string{"platform"}, []string{"Auth"},
			daysAgo(30), daysAgo(10),
			"Basic liveness probe for k8s readiness checks.", 1,
		},
		{
			"Customer requests data export (GDPR)", "11", "2", "2",
			"sarah", "jordan", []string{"gdpr", "customer"}, []string{"Compliance"},
			daysAgo(2), hoursAgo(20),
			"Legal approved; preparing the export bundle.", 3,
		},
		{
			"CDN cache purge script intermittently fails", "13", "3", "3",
			"morgan", "alex", []string{"bug", "cdn"}, []string{"Platform"},
			daysAgo(7), daysAgo(2),
			"Times out on large invalidation sets; needs batching.", 5,
		},
		{
			"Rotate signing key for JWTs", "12", "1", "2",
			"demo-user", "sarah", []string{"security"}, []string{"Auth"},
			daysAgo(1), daysAgo(1),
			"Quarterly rotation; coordinate with the mobile team.", 2,
		},
		{
			"Customer 3127 asks to increase webhook timeout", "11", "4", "4",
			"jordan", "riley", []string{"customer"}, []string{"API"},
			daysAgo(14), daysAgo(9),
			"Bumped to 30s per their request.", 1,
		},
	}

	for _, c := range cases {
		c := c
		s.CreateIssue(func(i *entIssue) {
			i.Summary = c.summary
			i.TypeID = c.typeID
			i.StatusID = c.statusID
			i.PriorityID = c.priorityID
			i.AssigneeID = c.assignee
			i.ReporterID = c.reporter
			i.Labels = c.labels
			i.Components = c.components
			i.Created = c.created
			i.Updated = c.updated
			i.Description = adfDoc(c.description)
			i.Customs = map[string]any{CFStoryPoints: c.storyPts}
		})
	}
}
