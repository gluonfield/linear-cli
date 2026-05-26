package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gluonfield/linear-cli/api"
)

var linearURLRe = regexp.MustCompile(`linear\.app/[^/]+/issue/([A-Za-z]+-\d+)`)
var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func resolveProjectID(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("project is empty")
	}
	if uuidRe.MatchString(input) {
		return input, nil
	}
	q := fmt.Sprintf(`query { projects(filter: { name: { containsIgnoreCase: "%s" } }, first: 50) { nodes { id name } } }`, escapeGraphQL(input))
	var res struct {
		Projects struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"projects"`
	}
	if err := api.Query(q, &res); err != nil {
		return "", err
	}
	nodes := res.Projects.Nodes
	for _, n := range nodes {
		if strings.EqualFold(n.Name, input) {
			return n.ID, nil
		}
	}
	if len(nodes) == 0 {
		return "", fmt.Errorf("project %q not found", input)
	}
	if len(nodes) == 1 {
		return nodes[0].ID, nil
	}
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return "", fmt.Errorf("project %q matched multiple: %s", input, strings.Join(names, ", "))
}

func parseIssueIdentifier(input string) string {
	input = strings.TrimSpace(input)
	if m := linearURLRe.FindStringSubmatch(input); len(m) > 1 {
		return m[1]
	}
	return input
}

func getViewerID() (string, error) {
	q := `query { viewer { id } }`
	var res struct {
		Viewer struct {
			ID string `json:"id"`
		} `json:"viewer"`
	}
	if err := api.Query(q, &res); err != nil {
		return "", err
	}
	return res.Viewer.ID, nil
}

func getViewerName() (string, error) {
	q := `query { viewer { name } }`
	var res struct {
		Viewer struct {
			Name string `json:"name"`
		} `json:"viewer"`
	}
	if err := api.Query(q, &res); err != nil {
		return "", err
	}
	return res.Viewer.Name, nil
}

func getViewerEmail() (string, error) {
	q := `query { viewer { email } }`
	var res struct {
		Viewer struct {
			Email string `json:"email"`
		} `json:"viewer"`
	}
	if err := api.Query(q, &res); err != nil {
		return "", err
	}
	return res.Viewer.Email, nil
}

func isMeAlias(input string) bool {
	return strings.EqualFold(strings.TrimSpace(input), "me")
}

func fetchTeamStates(teamID string) ([]struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}, error) {
	q := fmt.Sprintf(`query { workflowStates(filter: { team: { id: { eq: "%s" } } }) { nodes { id name } } }`, teamID)
	var res struct {
		WorkflowStates struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := api.Query(q, &res); err != nil {
		return nil, err
	}
	return res.WorkflowStates.Nodes, nil
}

func fuzzyMatchState(states []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}, input string) (string, string, error) {
	lower := strings.ToLower(input)
	var exactMatch struct {
		ID   string
		Name string
	}
	var partialMatches []struct {
		ID   string
		Name string
	}

	for _, s := range states {
		sl := strings.ToLower(s.Name)
		if sl == lower {
			exactMatch.ID = s.ID
			exactMatch.Name = s.Name
		}
		if strings.Contains(sl, lower) {
			partialMatches = append(partialMatches, struct {
				ID   string
				Name string
			}{s.ID, s.Name})
		}
	}

	if exactMatch.ID != "" {
		return exactMatch.ID, exactMatch.Name, nil
	}

	if len(partialMatches) == 1 {
		return partialMatches[0].ID, partialMatches[0].Name, nil
	}

	if len(partialMatches) > 1 {
		names := make([]string, len(partialMatches))
		for i, m := range partialMatches {
			names[i] = m.Name
		}
		return "", "", fmt.Errorf("status %q matched multiple states: %s", input, strings.Join(names, ", "))
	}

	names := make([]string, len(states))
	for i, s := range states {
		names[i] = s.Name
	}
	return "", "", fmt.Errorf("status %q not found. Available: %s", input, strings.Join(names, ", "))
}

func formatAvailableStates(states []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}) string {
	names := make([]string, len(states))
	for i, s := range states {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}
