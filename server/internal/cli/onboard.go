package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/config"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/spf13/cobra"
)

var (
	skipConfirm   bool
	orgTitle      string
	adminName     string
	adminEmail    string
	adminPassword string
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Initial setup wizard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Importinvoices Onboarding Wizard")
		fmt.Println("This process will initialize your database and set up the administrator account.")
		fmt.Println("If a database already exists, it will be migrated to the latest version.")
		fmt.Println()

		if !skipConfirm {
			fmt.Print("Do you want to continue? [y/N]: ")
			var confirm string
			_, _ = fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("Onboarding cancelled.")
				return nil
			}
		}

		cfg, err := config.Resolve(dataDir)
		if err != nil {
			return fmt.Errorf("failed to resolve config: %w", err)
		}

		reader := bufio.NewReader(os.Stdin)

		if orgTitle == "" {
			fmt.Print("Organization title [My Company]: ")
			line, _ := reader.ReadString('\n')
			orgTitle = strings.TrimSpace(line)
			if orgTitle == "" {
				orgTitle = "My Company"
			}
		}

		if adminName == "" {
			fmt.Print("Admin name [Admin]: ")
			line, _ := reader.ReadString('\n')
			adminName = strings.TrimSpace(line)
			if adminName == "" {
				adminName = "Admin"
			}
		}

		if adminEmail == "" {
			for {
				fmt.Print("Admin email: ")
				line, _ := reader.ReadString('\n')
				adminEmail = strings.TrimSpace(line)
				if adminEmail != "" {
					break
				}
				fmt.Println("Email is required.")
			}
		}

		if adminPassword == "" {
			for {
				fmt.Print("Admin password: ")
				line, _ := reader.ReadString('\n')
				adminPassword = strings.TrimSpace(line)
				if adminPassword == "" {
					fmt.Println("Password is required.")
					continue
				}
				if err := service.ValidatePassword(adminPassword); err != nil {
					fmt.Println(err.Error())
					adminPassword = ""
					continue
				}
				break
			}
		} else if err := service.ValidatePassword(adminPassword); err != nil {
			return err
		}

		store, err := db.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer func() { _ = store.Close() }()

		if err := store.Migrate(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}

		svc := service.New(store, nil, nil)
		
		needsSetup, err := svc.NeedsSetup(context.Background())
		if err != nil {
			return err
		}
		if !needsSetup {
			fmt.Println("System is already set up.")
			return nil
		}

		if err := svc.Setup(context.Background(), orgTitle, adminName, adminEmail, adminPassword); err != nil {
			return fmt.Errorf("setup failed: %w", err)
		}

		fmt.Println()
		fmt.Println("Setup completed successfully!")
		fmt.Printf("  Data dir: %s\n", cfg.DataDir)
		fmt.Printf("  Database: %s\n", cfg.DBPath)
		fmt.Println("  Run: importinvoices serve")
		return nil
	},
}

func init() {
	onboardCmd.Flags().StringVar(&orgTitle, "org", "", "Organization title")
	onboardCmd.Flags().StringVar(&adminName, "name", "", "Admin user name")
	onboardCmd.Flags().StringVar(&adminEmail, "email", "", "Admin user email")
	onboardCmd.Flags().StringVar(&adminPassword, "password", "", "Admin user password")
	onboardCmd.Flags().BoolVarP(&skipConfirm, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(onboardCmd)
}
