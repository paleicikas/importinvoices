package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/paleicikas/importinvoices/server/internal/config"
	"github.com/paleicikas/importinvoices/server/internal/db"
	"github.com/paleicikas/importinvoices/server/internal/service"
	"github.com/spf13/cobra"
)

var (
	resetEmail    string
	resetPassword string
)

var resetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "Reset a user's password (admin recovery)",
	Long: `Reset a user's password directly in the database, bypassing the
web UI. Useful for self-hosted recovery when an admin loses access.
All existing sessions for the user are invalidated.

The user is identified by --email. The new password can be supplied
with --password; otherwise you will be prompted interactively.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Resolve(dataDir)
		if err != nil {
			return fmt.Errorf("failed to resolve config: %w", err)
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

		email := strings.ToLower(strings.TrimSpace(resetEmail))
		if email == "" {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("User email: ")
			line, _ := reader.ReadString('\n')
			email = strings.ToLower(strings.TrimSpace(line))
		}
		if email == "" {
			return errors.New("email is required")
		}

		user, err := svc.GetUserByEmail(context.Background(), email)
		if err != nil {
			return fmt.Errorf("user not found: %w", err)
		}

		password := resetPassword
		if password == "" {
			reader := bufio.NewReader(os.Stdin)
			for {
				fmt.Print("New password: ")
				line, _ := reader.ReadString('\n')
				password = strings.TrimSpace(line)
				if password == "" {
					fmt.Println("Password is required.")
					continue
				}
				if err := service.ValidatePassword(password); err != nil {
					fmt.Println(err.Error())
					continue
				}
				break
			}
		} else if err := service.ValidatePassword(password); err != nil {
			return err
		}

		if err := svc.UpdatePassword(context.Background(), user.ID, password); err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}

		fmt.Printf("Password updated for %s (%s). All sessions have been invalidated.\n", user.Email, user.ID)
		return nil
	},
}

func init() {
	resetPasswordCmd.Flags().StringVar(&resetEmail, "email", "", "Email of the user whose password to reset")
	resetPasswordCmd.Flags().StringVar(&resetPassword, "password", "", "New password (prompted if omitted)")
	rootCmd.AddCommand(resetPasswordCmd)
}
