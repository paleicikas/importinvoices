package cli

import (
	"github.com/spf13/cobra"
)

var (
	dataDir string
)

var rootCmd = &cobra.Command{
	Use:   "importinvoices",
	Short: "Importinvoices - installable invoice management system",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Data directory (default is ~/.importinvoices)")
}
