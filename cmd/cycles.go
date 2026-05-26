package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

var cyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "List cycles",
	RunE: func(cmd *cobra.Command, args []string) error {
		teamFilter := ""
		if cmd.Flags().Changed("team") {
			teamFilter = fmt.Sprintf(`, filter: { team: { key: { eq: "%s" } } }`, cycleTeamFilter)
		}

		q := fmt.Sprintf(`query { cycles(first: 50%s) { nodes { id name number description isActive startsAt endsAt progress team { key } } } }`, teamFilter)

		var result struct {
			Cycles struct {
				Nodes []struct {
					ID          string  `json:"id"`
					Name        string  `json:"name"`
					Number      int     `json:"number"`
					Description string  `json:"description"`
					IsActive    bool    `json:"isActive"`
					StartsAt    string  `json:"startsAt"`
					EndsAt      string  `json:"endsAt"`
					Progress    float64 `json:"progress"`
					Team        *struct {
						Key string `json:"key"`
					} `json:"team"`
				} `json:"nodes"`
			} `json:"cycles"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.Cycles.Nodes
		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No cycles found.")
			return nil
		}

		return outputListItems(toAnySlice(nodes), func(item any) string {
			if n, ok := item.(struct {
				Name     string  `json:"name"`
				Progress float64 `json:"progress"`
			}); ok {
				return n.Name
			}
			return ""
		}, []string{"name", "team.key", "isActive", "progress"}, func() {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTEAM\tACTIVE\tPROGRESS")
			for _, c := range nodes {
				team := "-"
				if c.Team != nil {
					team = c.Team.Key
				}
				active := "no"
				if c.IsActive {
					active = "yes"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%.0f%%\n", c.Name, team, active, c.Progress*100)
			}
			w.Flush()
		})
	},
}

var (
	cycleTeamFilter string
	cycleCreateName string
	cycleCreateTeam string
	cycleStartDate  string
	cycleEndDate    string
	cycleCreateDesc string
)

var cycleCreateCmd = &cobra.Command{
	Use:   "cycle-create",
	Short: "Create a cycle",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cycleCreateName == "" {
			return fmt.Errorf("--name is required")
		}
		if cycleCreateTeam == "" {
			return fmt.Errorf("--team is required")
		}

		teamID, err := resolveTeamID(cycleCreateTeam)
		if err != nil {
			return err
		}

		input := fmt.Sprintf(`name: "%s", teamId: "%s"`, escapeGraphQL(cycleCreateName), teamID)
		if cycleCreateDesc != "" {
			input += fmt.Sprintf(`, description: "%s"`, escapeGraphQL(cycleCreateDesc))
		}
		if cycleStartDate != "" {
			input += fmt.Sprintf(`, startsAt: "%s"`, cycleStartDate)
		}
		if cycleEndDate != "" {
			input += fmt.Sprintf(`, endsAt: "%s"`, cycleEndDate)
		}

		q := fmt.Sprintf(`mutation { cycleCreate(input: { %s }) { cycle { id name number } } }`, input)

		var result struct {
			CycleCreate struct {
				Cycle struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					Number int    `json:"number"`
				} `json:"cycle"`
			} `json:"cycleCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		c := result.CycleCreate.Cycle

		switch effectiveFormat() {
		case "json":
			return writeJSON(c)
		case "id-only":
			fmt.Println(c.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", c.Name, c.ID)
			return nil
		}

		fmt.Printf("Created cycle: %s (%s)\n", c.Name, c.ID)
		return nil
	},
}

func init() {
	cyclesCmd.Flags().StringVarP(&cycleTeamFilter, "team", "t", "", "filter by team key")

	cycleCreateCmd.Flags().StringVarP(&cycleCreateName, "name", "n", "", "cycle name (required)")
	cycleCreateCmd.Flags().StringVarP(&cycleCreateTeam, "team", "t", "", "team key (required)")
	cycleCreateCmd.Flags().StringVar(&cycleCreateDesc, "desc", "", "description")
	cycleCreateCmd.Flags().StringVar(&cycleStartDate, "start", "", "start date (ISO 8601)")
	cycleCreateCmd.Flags().StringVar(&cycleEndDate, "end", "", "end date (ISO 8601)")

	rootCmd.AddCommand(cyclesCmd)
	rootCmd.AddCommand(cycleCreateCmd)
}
