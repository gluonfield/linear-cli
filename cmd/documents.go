package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "List documents",
	RunE: func(cmd *cobra.Command, args []string) error {
		filter := ""
		if docTeamFilter != "" {
			filter = fmt.Sprintf(`, filter: { team: { key: { eq: "%s" } } }`, docTeamFilter)
		}

		q := fmt.Sprintf(`query { documents(first: 50%s) { nodes { id title slugId content updatedAt createdAt project { name } } } }`, filter)

		var result struct {
			Documents struct {
				Nodes []struct {
					ID        string `json:"id"`
					Title     string `json:"title"`
					SlugID    string `json:"slugId"`
					Content   string `json:"content"`
					UpdatedAt string `json:"updatedAt"`
					CreatedAt string `json:"createdAt"`
					Project   *struct {
						Name string `json:"name"`
					} `json:"project"`
				} `json:"nodes"`
			} `json:"documents"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.Documents.Nodes
		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No documents found.")
			return nil
		}

		return outputListItems(toAnySlice(nodes), func(item any) string {
			if n, ok := item.(struct {
				Title  string `json:"title"`
				SlugID string `json:"slugId"`
			}); ok {
				return n.SlugID + "\t" + n.Title
			}
			return ""
		}, []string{"slugId", "title", "project.name"}, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "TITLE\tPROJECT\tUPDATED")
			for _, d := range nodes {
				project := "-"
				if d.Project != nil {
					project = d.Project.Name
				}
				updated := d.CreatedAt[:10]
				if d.UpdatedAt != "" {
					updated = d.UpdatedAt[:10]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", d.Title, project, updated)
			}
			w.Flush()
		})
	},
}

var (
	docTeamFilter  string
	docCreateTitle string
	docCreateDesc  string
	docCreateTeam  string
)

var docCreateCmd = &cobra.Command{
	Use:   "doc-create",
	Short: "Create a document",
	RunE: func(cmd *cobra.Command, args []string) error {
		if docCreateTitle == "" {
			return fmt.Errorf("--title is required")
		}

		input := fmt.Sprintf(`title: "%s", content: "%s"`, escapeGraphQL(docCreateTitle), escapeGraphQL(docCreateDesc))
		if docCreateTeam != "" {
			teamID, err := resolveTeamID(docCreateTeam)
			if err != nil {
				return err
			}
			input += fmt.Sprintf(`, teamId: "%s"`, teamID)
		}

		q := fmt.Sprintf(`mutation { documentCreate(input: { %s }) { document { id title url } } }`, input)

		var result struct {
			DocumentCreate struct {
				Document struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					URL   string `json:"url"`
				} `json:"document"`
			} `json:"documentCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		d := result.DocumentCreate.Document

		switch effectiveFormat() {
		case "json":
			return writeJSON(d)
		case "id-only":
			fmt.Println(d.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", d.Title, d.URL)
			return nil
		}

		fmt.Printf("Created: %s\nURL: %s\n", d.Title, d.URL)
		return nil
	},
}

func init() {
	docsCmd.Flags().StringVarP(&docTeamFilter, "team", "t", "", "filter by team key")

	docCreateCmd.Flags().StringVarP(&docCreateTitle, "title", "n", "", "document title (required)")
	docCreateCmd.Flags().StringVarP(&docCreateDesc, "desc", "d", "", "document content")
	docCreateCmd.Flags().StringVarP(&docCreateTeam, "team", "t", "", "team key")

	rootCmd.AddCommand(docsCmd)
	rootCmd.AddCommand(docCreateCmd)
}
