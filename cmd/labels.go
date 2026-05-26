package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

var labelsCmd = &cobra.Command{
	Use:   "labels",
	Short: "List issue labels",
	RunE: func(cmd *cobra.Command, args []string) error {
		teamFilter := ""
		if cmd.Flags().Changed("team") {
			teamFilter = fmt.Sprintf(`, filter: { team: { key: { eq: "%s" } } }`, labelTeamFilter)
		}

		q := fmt.Sprintf(`query { issueLabels(first: 100%s) { nodes { id name color isGroup } } }`, teamFilter)

		var result struct {
			IssueLabels struct {
				Nodes []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Color   string `json:"color"`
					IsGroup bool   `json:"isGroup"`
				} `json:"nodes"`
			} `json:"issueLabels"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.IssueLabels.Nodes
		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No labels found.")
			return nil
		}

		return outputListItems(toAnySlice(nodes), func(item any) string {
			if n, ok := item.(struct {
				Name string `json:"name"`
				ID   string `json:"id"`
			}); ok {
				return n.Name + "\t" + n.ID
			}
			return ""
		}, []string{"name", "id", "color"}, func() {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tID\tCOLOR\tGROUP")
			for _, l := range nodes {
				group := ""
				if l.IsGroup {
					group = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", l.Name, l.ID, l.Color, group)
			}
			w.Flush()
		})
	},
}

var (
	labelTeamFilter  string
	labelCreateName  string
	labelCreateColor string
	labelCreateTeam  string
	labelParentID    string
)

var labelCreateCmd = &cobra.Command{
	Use:   "label-create",
	Short: "Create an issue label",
	RunE: func(cmd *cobra.Command, args []string) error {
		if labelCreateName == "" {
			return fmt.Errorf("--name is required")
		}
		if labelCreateTeam == "" {
			return fmt.Errorf("--team is required")
		}

		teamID, err := resolveTeamID(labelCreateTeam)
		if err != nil {
			return err
		}

		input := fmt.Sprintf(`name: "%s", teamId: "%s", color: "%s"`, escapeGraphQL(labelCreateName), teamID, labelCreateColor)
		if labelParentID != "" {
			input += fmt.Sprintf(`, parentGroupId: "%s"`, labelParentID)
		}

		q := fmt.Sprintf(`mutation { issueLabelCreate(input: { %s }) { issueLabel { id name color } } }`, input)

		var result struct {
			IssueLabelCreate struct {
				IssueLabel struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Color string `json:"color"`
				} `json:"issueLabel"`
			} `json:"issueLabelCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		l := result.IssueLabelCreate.IssueLabel

		switch effectiveFormat() {
		case "json":
			return writeJSON(l)
		case "id-only":
			fmt.Println(l.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", l.Name, l.ID)
			return nil
		}

		fmt.Printf("Created label: %s (%s) color=%s\n", l.Name, l.ID, l.Color)
		return nil
	},
}

var labelDeleteCmd = &cobra.Command{
	Use:   "label-delete [label-id]",
	Short: "Delete an issue label",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		q := fmt.Sprintf(`mutation { issueLabelDelete(id: "%s") { success } }`, id)

		var result struct {
			IssueLabelDelete struct {
				Success bool `json:"success"`
			} `json:"issueLabelDelete"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}
		if !result.IssueLabelDelete.Success {
			return fmt.Errorf("delete failed")
		}

		switch effectiveFormat() {
		case "json":
			return writeJSON(map[string]any{"success": true, "id": id})
		case "id-only":
			fmt.Println(id)
			return nil
		}
		if optQuiet {
			fmt.Println(id)
			return nil
		}

		fmt.Printf("Deleted label: %s\n", id)
		return nil
	},
}

func init() {
	labelsCmd.Flags().StringVarP(&labelTeamFilter, "team", "t", "", "filter by team key")

	labelCreateCmd.Flags().StringVarP(&labelCreateName, "name", "n", "", "label name (required)")
	labelCreateCmd.Flags().StringVar(&labelCreateColor, "color", "#555555", "hex color")
	labelCreateCmd.Flags().StringVarP(&labelCreateTeam, "team", "t", "", "team key (required)")
	labelCreateCmd.Flags().StringVar(&labelParentID, "parent", "", "parent group ID")

	rootCmd.AddCommand(labelsCmd)
	rootCmd.AddCommand(labelCreateCmd)
	rootCmd.AddCommand(labelDeleteCmd)
}
