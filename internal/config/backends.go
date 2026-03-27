package config

// JiraConfig holds Jira-specific workspace configuration.
// These fields are populated from the "jira" key in a workspace's
// YAML config block.
type JiraConfig struct {
	BoardID      int               `yaml:"board_id"`
	ProjectKey   string            `yaml:"project_key"`
	TeamUUID     string            `yaml:"team_uuid,omitempty"`
	JQL          string            `yaml:"jql"`
	Filters      map[string]string `yaml:"filters,omitempty"`
	CustomFields map[string]int    `yaml:"custom_fields,omitempty"`
}

// GitHubConfig holds GitHub Issues/Projects workspace configuration.
// These fields are populated from the "github" key in a workspace's
// YAML config block.
type GitHubConfig struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
}

// TrelloConfig holds Trello workspace configuration.
// These fields are populated from the "trello" key in a workspace's
// YAML config block.
type TrelloConfig struct {
	BoardID string `yaml:"board_id"`
}
