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

