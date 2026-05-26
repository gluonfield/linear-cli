package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

var statesCmd = &cobra.Command{
	Use:   "states",
	Short: "List workflow states (statuses)",
	RunE: func(cmd *cobra.Command, args []string) error {
		teamFilter := ""
		if cmd.Flags().Changed("team") {
			teamFilter = fmt.Sprintf(`, filter: { team: { key: { eq: "%s" } } }`, stateTeamFilter)
		}

		q := fmt.Sprintf(`query { workflowStates(first: 100%s) { nodes { id name type color position team { key } } } }`, teamFilter)

		var result struct {
			WorkflowStates struct {
				Nodes []struct {
					ID       string  `json:"id"`
					Name     string  `json:"name"`
					Type     string  `json:"type"`
					Color    string  `json:"color"`
					Position float64 `json:"position"`
					Team     *struct {
						Key string `json:"key"`
					} `json:"team"`
				} `json:"nodes"`
			} `json:"workflowStates"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.WorkflowStates.Nodes
		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No states found.")
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
		}, []string{"name", "type", "team.key", "id"}, func() {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tTEAM\tCOLOR\tID")
			for _, s := range nodes {
				team := "-"
				if s.Team != nil {
					team = s.Team.Key
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.Type, team, s.Color, s.ID)
			}
			w.Flush()
		})
	},
}

var (
	stateTeamFilter  string
	stateCreateName  string
	stateCreateType  string
	stateCreateTeam  string
	stateCreateColor string
)

var stateCreateCmd = &cobra.Command{
	Use:   "state-create",
	Short: "Create a workflow state",
	RunE: func(cmd *cobra.Command, args []string) error {
		if stateCreateName == "" {
			return fmt.Errorf("--name is required")
		}
		if stateCreateTeam == "" {
			return fmt.Errorf("--team is required")
		}
		if stateCreateType == "" {
			stateCreateType = "backlog"
		}

		teamID, err := resolveTeamID(stateCreateTeam)
		if err != nil {
			return err
		}

		input := fmt.Sprintf(`name: "%s", type: "%s", teamId: "%s"`, escapeGraphQL(stateCreateName), stateCreateType, teamID)
		if stateCreateColor != "" {
			input += fmt.Sprintf(`, color: "%s"`, stateCreateColor)
		}

		q := fmt.Sprintf(`mutation { workflowStateCreate(input: { %s }) { workflowState { id name type color } } }`, input)

		var result struct {
			WorkflowStateCreate struct {
				WorkflowState struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					Type  string `json:"type"`
					Color string `json:"color"`
				} `json:"workflowState"`
			} `json:"workflowStateCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		s := result.WorkflowStateCreate.WorkflowState

		switch effectiveFormat() {
		case "json":
			return writeJSON(s)
		case "id-only":
			fmt.Println(s.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", s.Name, s.ID)
			return nil
		}

		fmt.Printf("Created state: %s (%s) type=%s\n", s.Name, s.ID, s.Type)
		return nil
	},
}

func init() {
	statesCmd.Flags().StringVarP(&stateTeamFilter, "team", "t", "", "filter by team key")

	stateCreateCmd.Flags().StringVarP(&stateCreateName, "name", "n", "", "state name (required)")
	stateCreateCmd.Flags().StringVar(&stateCreateType, "type", "backlog", "type (started, completed, canceled, backlog, triage, unstarted)")
	stateCreateCmd.Flags().StringVarP(&stateCreateTeam, "team", "t", "", "team key (required)")
	stateCreateCmd.Flags().StringVar(&stateCreateColor, "color", "", "hex color")

	rootCmd.AddCommand(statesCmd)
	rootCmd.AddCommand(stateCreateCmd)
}
