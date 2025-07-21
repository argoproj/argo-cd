package commands

import (
	stderrors "errors"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

type bcryptCmdOpt struct {
	password    string
	compareHash string
}

// NewBcryptCmd represents the bcrypt command
func NewBcryptCmd() *cobra.Command {
	var opt bcryptCmdOpt

	bcryptCmd := &cobra.Command{
		Use:   "bcrypt",
		Short: "Generate bcrypt hash for any password",
		Example: `# Generate bcrypt hash for any password 
argocd account bcrypt --password YOUR_PASSWORD
argocd account bcrypt --password YOUR_PASSWORD --compare 'YOUR_HASH_PASSWORD'`,
		Run: func(cmd *cobra.Command, _ []string) {
			out, err := runBcryptCommand(opt)
			if err != nil {
				errors.CheckError(err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(out))
		},
	}

	bcryptCmd.Flags().StringVar(&opt.password, "password", "", "Password for which bcrypt hash is generated")
	bcryptCmd.Flags().StringVar(&opt.compareHash, "compare", "", "Existing bcrypt hash to compare with password")
	err := bcryptCmd.MarkFlagRequired("password")
	if err != nil {
		return nil
	}
	return bcryptCmd
}

func runBcryptCommand(opt bcryptCmdOpt) (string, error) {
	bytePassword := []byte(opt.password)
	byteCompareHash := []byte(opt.compareHash)

	// Compare hash password and password
	if len(byteCompareHash) > 0 && len(bytePassword) > 0 {
		err := bcrypt.CompareHashAndPassword(byteCompareHash, bytePassword)
		if err != nil {
			if stderrors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
				return "no", nil
			}
			return "", fmt.Errorf("failed to compare password: %w", err)
		}
		return "yes", nil
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword(bytePassword, bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to generate bcrypt hash: %w", err)
	}
	return string(hash), nil
}
