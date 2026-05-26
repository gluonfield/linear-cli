package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var getCmd = &cobra.Command{
	Use:   "get [issue-id]",
	Short: "Get issue details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := parseIssueIdentifier(args[0])
		q := fmt.Sprintf(`query { issue(id: "%s") { id identifier title description state { name } assignee { name } priority labels { nodes { name } } team { key name } project { id name } url createdAt updatedAt comments(first: 100) { nodes { id body user { name } createdAt updatedAt resolvedAt } } } }`, id)

		var result struct {
			Issue *struct {
				ID          string `json:"id"`
				Identifier  string `json:"identifier"`
				Title       string `json:"title"`
				Description string `json:"description"`
				State       *struct {
					Name string `json:"name"`
				} `json:"state"`
				Assignee *struct {
					Name string `json:"name"`
				} `json:"assignee"`
				Priority int `json:"priority"`
				Labels   *struct {
					Nodes []struct {
						Name string `json:"name"`
					} `json:"nodes"`
				} `json:"labels"`
				Team *struct {
					Key  string `json:"key"`
					Name string `json:"name"`
				} `json:"team"`
				Project *struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"project"`
				URL       string `json:"url"`
				CreatedAt string `json:"createdAt"`
				UpdatedAt string `json:"updatedAt"`
				Comments  *struct {
					Nodes []struct {
						ID    string `json:"id"`
						Body  string `json:"body"`
						User  *struct {
							Name string `json:"name"`
						} `json:"user"`
						CreatedAt  string  `json:"createdAt"`
						UpdatedAt  string  `json:"updatedAt"`
						ResolvedAt *string `json:"resolvedAt"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"issue"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}
		if result.Issue == nil {
			return fmt.Errorf("issue %q not found", id)
		}

		issue := result.Issue

		switch effectiveFormat() {
		case "json":
			return writeJSON(issue)
		case "id-only":
			fmt.Println(issue.Identifier)
			return nil
		}

		if optFields != "" {
			fields := parseFields(optFields)
			m := toMap(issue)
			tsvPrint(fields...)
			row := make([]string, len(fields))
			for i, f := range fields {
				row[i] = fieldStr(getField(m, f))
			}
			tsvPrint(row...)
			return nil
		}

		if optQuiet {
			fmt.Printf("%s\t%s\n", issue.Identifier, issue.URL)
			return nil
		}

		fmt.Printf("%s - %s\n", issue.Identifier, issue.Title)
		fmt.Printf("URL: %s\n", issue.URL)
		if issue.Team != nil {
			fmt.Printf("Team: %s (%s)\n", issue.Team.Name, issue.Team.Key)
		}
		if issue.Project != nil {
			fmt.Printf("Project: %s\n", issue.Project.Name)
		}
		state := "-"
		if issue.State != nil {
			state = issue.State.Name
		}
		fmt.Printf("Status: %s\n", state)
		assignee := "-"
		if issue.Assignee != nil {
			assignee = issue.Assignee.Name
		}
		fmt.Printf("Assignee: %s\n", assignee)
		fmt.Printf("Priority: %s\n", priorityLabel(issue.Priority))
		if issue.Labels != nil && len(issue.Labels.Nodes) > 0 {
			fmt.Print("Labels: ")
			for i, l := range issue.Labels.Nodes {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(l.Name)
			}
			fmt.Println()
		}
		fmt.Printf("Created: %s\n", issue.CreatedAt)
		fmt.Printf("Updated: %s\n", issue.UpdatedAt)
		if issue.Description != "" {
			fmt.Printf("\n%s\n", issue.Description)
		}
		if issue.Comments != nil && len(issue.Comments.Nodes) > 0 {
			fmt.Printf("\n--- Comments (%d) ---\n", len(issue.Comments.Nodes))
			for _, c := range issue.Comments.Nodes {
				user := "unknown"
				if c.User != nil {
					user = c.User.Name
				}
				resolved := ""
				if c.ResolvedAt != nil {
					resolved = " [resolved]"
				}
				date := c.CreatedAt
				if len(date) >= 10 {
					date = date[:10]
				}
				fmt.Printf("\n[%s] %s%s\n%s\n", date, user, resolved, c.Body)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(getCmd)
	getCmd.Flags().StringVar(&optFields, "fields", "", "comma-separated fields (e.g. identifier,title,state.name)")
}
