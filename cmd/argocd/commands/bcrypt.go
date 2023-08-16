package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCmd represents the bcrypt command
func NewBcryptCmd() *cobra.Command {
	var (
		password string
	)
	var bcryptCmd = &cobra.Command{
		Use:   "bcrypt",
		Short: "Generate bcrypt hash for the admin password",
		Run: func(cmd *cobra.Command, args []string) {
			bytePassword := []byte(password)
			// Hashing the password
			hash, err := bcrypt.GenerateFromPassword(bytePassword, bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Failed to genarate bcrypt hash: %v", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(hash))
		},
	}

	bcryptCmd.Flags().StringVar(&password, "password", "", "Password for which bcrypt hash is generated")
	err := bcryptCmd.MarkFlagRequired("password")
	if err != nil {
		return nil
	}
	return bcryptCmd
}
