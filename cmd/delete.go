package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var archiveFlag bool

var deleteCmd = &cobra.Command{
	Use:   "delete [issue-id]",
	Short: "Delete (trash) an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		q := fmt.Sprintf(`mutation { issueDelete(id: "%s") { success } }`, id)

		var result struct {
			IssueDelete struct {
				Success bool `json:"success"`
			} `json:"issueDelete"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}
		if !result.IssueDelete.Success {
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

		fmt.Printf("Deleted: %s\n", id)
		return nil
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive [issue-id]",
	Short: "Archive an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		q := fmt.Sprintf(`mutation { issueArchive(id: "%s") { success } }`, id)

		var result struct {
			IssueArchive struct {
				Success bool `json:"success"`
			} `json:"issueArchive"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}
		if !result.IssueArchive.Success {
			return fmt.Errorf("archive failed")
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

		fmt.Printf("Archived: %s\n", id)
		return nil
	},
}

var unarchiveCmd = &cobra.Command{
	Use:   "unarchive [issue-id]",
	Short: "Unarchive an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		q := fmt.Sprintf(`mutation { issueUnarchive(id: "%s") { success } }`, id)

		var result struct {
			IssueUnarchive struct {
				Success bool `json:"success"`
			} `json:"issueUnarchive"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}
		if !result.IssueUnarchive.Success {
			return fmt.Errorf("unarchive failed")
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

		fmt.Printf("Unarchived: %s\n", id)
		return nil
	},
}

func init() {
	deleteCmd.Flags().BoolVar(&archiveFlag, "archive", false, "archive instead of delete")
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(unarchiveCmd)
}
