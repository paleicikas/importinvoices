package cli

import (
	"fmt"

	"github.com/paleicikas/importinvoices/server/internal/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of importinvoices",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("importinvoices version %s\n", config.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
