package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var (
	createTitle       string
	createDesc        string
	createTeamKey     string
	createProject     string
	createPriority    int
	createStatusLabel string
	createAssignee    string
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an issue",
	RunE: func(cmd *cobra.Command, args []string) error {
		if createTitle == "" {
			return fmt.Errorf("--title is required")
		}
		if createTeamKey == "" {
			return fmt.Errorf("--team is required (e.g. ADI)")
		}

		teamID, err := resolveTeamID(createTeamKey)
		if err != nil {
			return err
		}

		inputFields := fmt.Sprintf(`title: "%s", teamId: "%s"`, escapeGraphQL(createTitle), teamID)
		if createDesc != "" {
			inputFields += fmt.Sprintf(`, description: "%s"`, escapeGraphQL(createDesc))
		}
		if createPriority > 0 && createPriority <= 4 {
			inputFields += fmt.Sprintf(", priority: %d", createPriority)
		}
		if createProject != "" {
			projectID, err := resolveProjectID(createProject)
			if err != nil {
				return err
			}
			inputFields += fmt.Sprintf(`, projectId: "%s"`, projectID)
		}
		if createAssignee != "" {
			assigneeID, err := resolveAssigneeID(createAssignee)
			if err != nil {
				return err
			}
			inputFields += fmt.Sprintf(`, assigneeId: "%s"`, assigneeID)
		}

		q := fmt.Sprintf(`mutation { issueCreate(input: { %s }) { issue { id identifier url title } } }`, inputFields)

		var result struct {
			IssueCreate struct {
				Issue struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
					URL        string `json:"url"`
					Title      string `json:"title"`
				} `json:"issue"`
			} `json:"issueCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		issue := result.IssueCreate.Issue

		switch effectiveFormat() {
		case "json":
			return writeJSON(issue)
		case "id-only":
			fmt.Println(issue.Identifier)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", issue.Identifier, issue.URL)
			return nil
		}

		fmt.Printf("Created: %s - %s\n", issue.Identifier, issue.Title)
		fmt.Printf("URL: %s\n", issue.URL)
		return nil
	},
}

func escapeGraphQL(s string) string {
	r := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			r = append(r, '\\', '"')
		case '\\':
			r = append(r, '\\', '\\')
		case '\n':
			r = append(r, '\\', 'n')
		case '\r':
			r = append(r, '\\', 'r')
		case '\t':
			r = append(r, '\\', 't')
		default:
			r = append(r, c)
		}
	}
	return string(r)
}

func resolveTeamID(key string) (string, error) {
	q := fmt.Sprintf(`query { teams(filter: { key: { eq: "%s" } }) { nodes { id key name } } }`, key)
	var result struct {
		Teams struct {
			Nodes []struct {
				ID   string `json:"id"`
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := api.Query(q, &result); err != nil {
		return "", err
	}
	if len(result.Teams.Nodes) == 0 {
		return "", fmt.Errorf("team %q not found", key)
	}
	return result.Teams.Nodes[0].ID, nil
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&createTitle, "title", "t", "", "issue title (required)")
	createCmd.Flags().StringVarP(&createDesc, "desc", "d", "", "issue description (markdown)")
	createCmd.Flags().StringVarP(&createTeamKey, "team", "T", "", "team key (e.g. ADI) (required)")
	createCmd.Flags().StringVar(&createProject, "project", "", "project name or UUID")
	createCmd.Flags().StringVarP(&createAssignee, "assignee", "a", "", "assign to user by name or 'me'")
	createCmd.Flags().IntVarP(&createPriority, "priority", "p", 0, "priority (1=urgent 2=high 3=medium 4=low)")
	createCmd.Flags().StringVar(&createStatusLabel, "status", "", "status name")
}
