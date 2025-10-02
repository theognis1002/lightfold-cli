package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"lightfold/pkg/ssh"
	"os"

	"github.com/spf13/cobra"
)

func generateRandomKeyName() (string, error) {
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("lightfold-%s", hex.EncodeToString(randomBytes)), nil
}

var keygenCmd = &cobra.Command{
	Use:   "keygen [KEY_NAME]",
	Short: "Generate a new SSH key pair",
	Long: `Generate a new Ed25519 SSH key pair for use with Lightfold deployments.

Keys are stored in ~/.lightfold/keys/ with secure permissions.
If no key name is provided, a random name will be generated.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var keyName string
		if len(args) == 0 {
			var err error
			keyName, err = generateRandomKeyName()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating random key name: %v\n", err)
				os.Exit(1)
			}
		} else {
			keyName = args[0]
		}

		// Check if key already exists
		exists, err := ssh.KeyExists(keyName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking key existence: %v\n", err)
			os.Exit(1)
		}

		if exists {
			fmt.Fprintf(os.Stderr, "Error: SSH key '%s' already exists\n", keyName)
			os.Exit(1)
		}

		// Generate the key pair
		fmt.Printf("Generating Ed25519 SSH key pair: %s\n", keyName)
		keyPair, err := ssh.GenerateKeyPair(keyName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating key pair: %v\n", err)
			os.Exit(1)
		}

		// Display success information
		fmt.Println("\n✓ SSH key pair generated successfully")
		fmt.Println()
		fmt.Printf("Private key: %s\n", keyPair.PrivateKeyPath)
		fmt.Printf("Public key:  %s\n", keyPair.PublicKeyPath)
		fmt.Printf("Fingerprint: %s\n", keyPair.Fingerprint)
		fmt.Println()
		fmt.Println("Public key content:")
		fmt.Println(keyPair.PublicKey)
	},
}

func init() {
	rootCmd.AddCommand(keygenCmd)
}
