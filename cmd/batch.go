package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var batchCmd = &cobra.Command{
	Use:   "batch-create",
	Short: "Batch create issues from stdin (JSON lines)",
	Long: `Read JSON lines from stdin, each with title (required), description, priority, project, assignee.
Team key is required via --team. Per-line "project" (name or UUID) overrides --project.

Example:
  echo '{"title":"Issue 1"}\n{"title":"Issue 2","priority":2,"project":"Q3 Launch"}' | linear batch-create --team ADI`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if batchTeamKey == "" {
			return fmt.Errorf("--team is required")
		}

		teamID, err := resolveTeamID(batchTeamKey)
		if err != nil {
			return err
		}

		var defaultProjectID string
		if batchProject != "" {
			defaultProjectID, err = resolveProjectID(batchProject)
			if err != nil {
				return err
			}
		}
		projectCache := map[string]string{}

		inputs := []map[string]interface{}{}
		decoder := json.NewDecoder(cmd.InOrStdin())
		for decoder.More() {
			var item map[string]interface{}
			if err := decoder.Decode(&item); err != nil {
				return fmt.Errorf("parse JSON line: %w", err)
			}
			inputs = append(inputs, item)
		}

		if len(inputs) == 0 {
			return fmt.Errorf("no input (pipe JSON lines)")
		}

		issues := []map[string]interface{}{}
		for _, item := range inputs {
			issue := map[string]interface{}{
				"title":  item["title"],
				"teamId": teamID,
			}
			if desc, ok := item["description"]; ok {
				issue["description"] = desc
			}
			if prio, ok := item["priority"]; ok {
				issue["priority"] = prio
			}
			projectInput, ok := item["project"].(string)
			if ok && projectInput != "" {
				pid, cached := projectCache[projectInput]
				if !cached {
					pid, err = resolveProjectID(projectInput)
					if err != nil {
						return fmt.Errorf("resolve project %q: %w", projectInput, err)
					}
					projectCache[projectInput] = pid
				}
				issue["projectId"] = pid
			} else if defaultProjectID != "" {
				issue["projectId"] = defaultProjectID
			}
			if assignee, ok := item["assignee"].(string); ok && assignee != "" {
				aid, err := resolveAssigneeID(assignee)
				if err != nil {
					return fmt.Errorf("resolve assignee %q: %w", assignee, err)
				}
				issue["assigneeId"] = aid
			}
			issues = append(issues, issue)
		}

		issuesJSON, err := json.Marshal(issues)
		if err != nil {
			return err
		}

		q := fmt.Sprintf(`mutation { issueBatchCreate(input: { issues: %s }) { issues { id identifier title } } }`, string(issuesJSON))

		var result struct {
			IssueBatchCreate struct {
				Issues []struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
					Title      string `json:"title"`
				} `json:"issues"`
			} `json:"issueBatchCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		created := result.IssueBatchCreate.Issues

		switch effectiveFormat() {
		case "json":
			return writeJSON(created)
		case "id-only":
			for _, i := range created {
				fmt.Println(i.Identifier)
			}
			return nil
		}
		if optQuiet {
			for _, i := range created {
				fmt.Printf("%s\t%s\n", i.Identifier, i.Title)
			}
			return nil
		}

		fmt.Printf("Created %d issues:\n", len(created))
		for _, i := range created {
			fmt.Printf("  %s - %s\n", i.Identifier, i.Title)
		}
		return nil
	},
}

var (
	batchTeamKey string
	batchProject string
)

func init() {
	batchCmd.Flags().StringVarP(&batchTeamKey, "team", "t", "", "team key (required)")
	batchCmd.Flags().StringVar(&batchProject, "project", "", "default project (name or UUID) for all issues")
	rootCmd.AddCommand(batchCmd)
}
