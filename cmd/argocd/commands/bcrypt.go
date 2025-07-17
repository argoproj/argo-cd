package commands

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

// NewBcryptCmd represents the bcrypt command
func NewBcryptCmd() *cobra.Command {
	var password string
	bcryptCmd := &cobra.Command{
		Use:   "bcrypt",
		Short: "Generate bcrypt hash for any password",
		Example: `# Generate bcrypt hash for any password 
argocd account bcrypt --password YOUR_PASSWORD`,
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
