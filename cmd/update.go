package cmd

import (
	"fmt"
	"strings"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

var (
	updateTitle        string
	updateDesc         string
	updateStatus       string
	updateAssignee     string
	updatePriority     int
	updateLabels       []string
	updateDueDate      string
	updateClearDue     bool
	updateProject      string
	updateClearProject bool
)

var updateCmd = &cobra.Command{
	Use:   "update [issue-id]",
	Short: "Update an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := parseIssueIdentifier(args[0])

		fields := []string{}
		if updateTitle != "" {
			fields = append(fields, fmt.Sprintf(`title: "%s"`, escapeGraphQL(updateTitle)))
		}
		if updateDesc != "" {
			fields = append(fields, fmt.Sprintf(`description: "%s"`, escapeGraphQL(updateDesc)))
		}
		if updatePriority > 0 && updatePriority <= 4 {
			fields = append(fields, fmt.Sprintf("priority: %d", updatePriority))
		}
		if updatePriority == -1 {
			fields = append(fields, "priority: null")
		}
		if updateStatus != "" {
			stateID, _, err := resolveStateIDFuzzy(id, updateStatus)
			if err != nil {
				return err
			}
			fields = append(fields, fmt.Sprintf(`stateId: "%s"`, stateID))
		}
		if updateAssignee != "" {
			assigneeID, err := resolveAssigneeID(updateAssignee)
			if err != nil {
				return err
			}
			fields = append(fields, fmt.Sprintf(`assigneeId: "%s"`, assigneeID))
		}
		if updateClearDue {
			fields = append(fields, "dueDate: null")
		} else if updateDueDate != "" {
			fields = append(fields, fmt.Sprintf(`dueDate: "%s"`, updateDueDate))
		}
		if updateClearProject {
			fields = append(fields, "projectId: null")
		} else if updateProject != "" {
			projectID, err := resolveProjectID(updateProject)
			if err != nil {
				return err
			}
			fields = append(fields, fmt.Sprintf(`projectId: "%s"`, projectID))
		}

		if len(fields) == 0 {
			return fmt.Errorf("no updates specified (use --title, --desc, --status, --assignee, --priority, --project)")
		}

		q := fmt.Sprintf(`mutation { issueUpdate(id: "%s", input: { %s }) { success issue { id identifier title state { name } assignee { name } priority } } }`, id, strings.Join(fields, ", "))

		var result struct {
			IssueUpdate struct {
				Success bool `json:"success"`
				Issue   struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
					Title      string `json:"title"`
					State      *struct {
						Name string `json:"name"`
					} `json:"state"`
					Assignee *struct {
						Name string `json:"name"`
					} `json:"assignee"`
					Priority int `json:"priority"`
				} `json:"issue"`
			} `json:"issueUpdate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		if !result.IssueUpdate.Success {
			return fmt.Errorf("update failed")
		}

		issue := result.IssueUpdate.Issue

		switch effectiveFormat() {
		case "json":
			return writeJSON(issue)
		case "id-only":
			fmt.Println(issue.Identifier)
			return nil
		}
		if optQuiet {
			fmt.Println(issue.Identifier)
			return nil
		}

		fmt.Printf("Updated: %s - %s\n", issue.Identifier, issue.Title)

		if updateLabels != nil {
			if err := updateIssueLabels(id, updateLabels); err != nil {
				return err
			}
		}

		return nil
	},
}

func resolveStateIDFuzzy(issueID, stateName string) (string, string, error) {
	q := fmt.Sprintf(`query { issue(id: "%s") { team { id } } }`, issueID)
	var issueRes struct {
		Issue *struct {
			Team *struct {
				ID string `json:"id"`
			} `json:"team"`
		} `json:"issue"`
	}
	if err := api.Query(q, &issueRes); err != nil {
		return "", "", err
	}
	if issueRes.Issue == nil || issueRes.Issue.Team == nil {
		return "", "", fmt.Errorf("could not resolve team for issue %s", issueID)
	}

	states, err := fetchTeamStates(issueRes.Issue.Team.ID)
	if err != nil {
		return "", "", err
	}
	return fuzzyMatchState(states, stateName)
}

func resolveAssigneeID(name string) (string, error) {
	if isMeAlias(name) {
		return getViewerID()
	}
	q := fmt.Sprintf(`query { users(filter: { name: { eq: "%s" } }) { nodes { id name } } }`, name)
	var res struct {
		Users struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"users"`
	}
	if err := api.Query(q, &res); err != nil {
		return "", err
	}
	if len(res.Users.Nodes) == 0 {
		return "", fmt.Errorf("user %q not found", name)
	}
	return res.Users.Nodes[0].ID, nil
}

func updateIssueLabels(issueID string, labels []string) error {
	for _, labelName := range labels {
		q := fmt.Sprintf(`mutation { issueAddLabel(input: { issueId: "%s", labelId: "%s" }) { success } }`, issueID, labelName)
		var res struct {
			IssueAddLabel struct {
				Success bool `json:"success"`
			} `json:"issueAddLabel"`
		}
		if err := api.Query(q, &res); err != nil {
			return fmt.Errorf("add label %q: %w", labelName, err)
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVarP(&updateTitle, "title", "t", "", "new title")
	updateCmd.Flags().StringVarP(&updateDesc, "desc", "d", "", "new description")
	updateCmd.Flags().StringVarP(&updateStatus, "status", "S", "", "new status name (e.g. 'In Progress')")
	updateCmd.Flags().StringVarP(&updateAssignee, "assignee", "a", "", "assign to user by name or 'me'")
	updateCmd.Flags().IntVarP(&updatePriority, "priority", "p", 0, "priority (1-4), -1 to clear")
	updateCmd.Flags().StringSliceVar(&updateLabels, "labels", nil, "add labels by ID")
	updateCmd.Flags().StringVar(&updateDueDate, "due", "", "due date (ISO 8601)")
	updateCmd.Flags().BoolVar(&updateClearDue, "clear-due", false, "clear due date")
	updateCmd.Flags().StringVar(&updateProject, "project", "", "move to project (name or UUID)")
	updateCmd.Flags().BoolVar(&updateClearProject, "clear-project", false, "remove from project")
}
