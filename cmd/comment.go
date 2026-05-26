package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var commentBody string

var commentCmd = &cobra.Command{
	Use:   "comment [issue-id] [body]",
	Short: "Add a comment to an issue",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := parseIssueIdentifier(args[0])

		body := commentBody
		if len(args) == 2 {
			body = args[1]
		}
		if body == "" {
			return fmt.Errorf("comment body is required (pass as arg or --body)")
		}

		q := fmt.Sprintf(`mutation { commentCreate(input: { issueId: "%s", body: "%s" }) { comment { id body createdAt } } }`, id, escapeGraphQL(body))

		var result struct {
			CommentCreate struct {
				Comment struct {
					ID        string `json:"id"`
					Body      string `json:"body"`
					CreatedAt string `json:"createdAt"`
				} `json:"comment"`
			} `json:"commentCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		c := result.CommentCreate.Comment

		switch effectiveFormat() {
		case "json":
			return writeJSON(c)
		case "id-only":
			fmt.Println(c.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", id, c.ID)
			return nil
		}

		fmt.Printf("Comment added to %s at %s\n", id, c.CreatedAt)
		return nil
	},
}

var listCommentsCmd = &cobra.Command{
	Use:   "comments [issue-id]",
	Short: "List comments on an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := parseIssueIdentifier(args[0])
		q := fmt.Sprintf(`query { issue(id: "%s") { comments { nodes { id body user { name } createdAt updatedAt resolvedAt } } } }`, id)

		var result struct {
			Issue *struct {
				Comments struct {
					Nodes []struct {
						ID         string  `json:"id"`
						Body       string  `json:"body"`
						User       *struct {
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

		nodes := result.Issue.Comments.Nodes

		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No comments.")
			return nil
		}

		switch effectiveFormat() {
		case "json":
			return writeJSON(nodes)
		case "id-only":
			for _, c := range nodes {
				fmt.Println(c.ID)
			}
			return nil
		}

		if optQuiet {
			for _, c := range nodes {
				user := "unknown"
				if c.User != nil {
					user = c.User.Name
				}
				body := strings.ReplaceAll(c.Body, "\n", " ")
				if len(body) > 80 {
					body = body[:80] + "..."
				}
				fmt.Printf("%s\t%s\t%s\n", c.ID, user, body)
			}
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "USER\tID\tRESOLVED\tDATE\tBODY")
		for _, c := range nodes {
			user := "unknown"
			if c.User != nil {
				user = c.User.Name
			}
			resolved := ""
			if c.ResolvedAt != nil {
				resolved = "yes"
			}
			body := strings.ReplaceAll(c.Body, "\n", " ")
			if len(body) > 60 {
				body = body[:60] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", user, c.ID, resolved, c.CreatedAt[:10], body)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(commentCmd)
	rootCmd.AddCommand(listCommentsCmd)
	commentCmd.Flags().StringVarP(&commentBody, "body", "b", "", "comment body (use for multiline)")
}
