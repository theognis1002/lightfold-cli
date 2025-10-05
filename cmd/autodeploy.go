package cmd

import (
	_ "embed"
	"fmt"
	"lightfold/pkg/config"
	"lightfold/pkg/util"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

//go:embed templates/github-workflow.yml.tmpl
var githubWorkflowTemplate string

var (
	autoDeployBranch    string
	autoDeployTarget    string
	autoDeployNoConfirm bool

	autoDeploySuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
	autoDeployMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	autoDeployValueStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))
	autoDeployErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	autoDeployLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
)

var autoDeployCmd = &cobra.Command{
	Use:   "autodeploy",
	Short: "Manage GitHub Actions auto-deployment",
	Long:  `Configure GitHub Actions to automatically deploy your application on git push.`,
}

var autoDeploySetupCmd = &cobra.Command{
	Use:   "setup [PROJECT_PATH]",
	Short: "Setup GitHub Actions auto-deploy workflow",
	Long: `Setup a GitHub Actions workflow to automatically deploy your application when pushing to a branch.

This command will:
- Detect your GitHub repository
- Create .github/workflows/lightfold-deploy.yml
- Provide instructions for setting up GitHub secrets

Examples:
  lightfold autodeploy setup                    # Setup for current directory
  lightfold autodeploy setup ~/Projects/myapp   # Setup for specific project
  lightfold autodeploy setup --target myapp     # Setup for named target`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfigOrExit()

		var pathArg string
		if len(args) > 0 {
			pathArg = args[0]
		}

		target, targetName := resolveTarget(cfg, autoDeployTarget, pathArg)
		projectPath := target.ProjectPath

		// Validate it's a Git repository
		if !util.IsGitRepository(projectPath) {
			fmt.Fprintf(os.Stderr, "%s\n", autoDeployErrorStyle.Render("Error: Not a git repository"))
			fmt.Fprintf(os.Stderr, "Initialize git first: git init\n")
			os.Exit(1)
		}

		// Get GitHub org/repo
		org, repo, err := util.GetGitHubRepo(projectPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", autoDeployErrorStyle.Render("Error: Not a GitHub repository"))
			fmt.Fprintf(os.Stderr, "Remote URL must be a GitHub repository.\n")
			fmt.Fprintf(os.Stderr, "Details: %v\n", err)
			os.Exit(1)
		}

		// Check if workflow already exists
		workflowPath := filepath.Join(projectPath, ".github", "workflows", "lightfold-deploy.yml")
		if _, err := os.Stat(workflowPath); err == nil {
			if !autoDeployNoConfirm {
				fmt.Printf("Workflow file already exists: %s\n", workflowPath)
				fmt.Print("Overwrite? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(strings.TrimSpace(response)) != "y" {
					fmt.Println("Cancelled")
					return
				}
			}
		}

		// Render the template
		rendered := githubWorkflowTemplate
		rendered = strings.ReplaceAll(rendered, "{{BRANCH}}", autoDeployBranch)
		rendered = strings.ReplaceAll(rendered, "{{TARGET_NAME}}", targetName)

		// Create .github/workflows directory if it doesn't exist
		workflowDir := filepath.Join(projectPath, ".github", "workflows")
		if err := os.MkdirAll(workflowDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", autoDeployErrorStyle.Render(fmt.Sprintf("Error creating workflow directory: %v", err)))
			os.Exit(1)
		}

		// Write the workflow file
		if err := os.WriteFile(workflowPath, []byte(rendered), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", autoDeployErrorStyle.Render(fmt.Sprintf("Error writing workflow file: %v", err)))
			os.Exit(1)
		}

		// Get the provider token for display
		tokens, err := config.LoadTokens()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", autoDeployErrorStyle.Render(fmt.Sprintf("Warning: Could not load tokens: %v", err)))
		}

		providerToken := ""
		providerName := target.Provider
		if tokens != nil {
			providerToken = tokens.GetToken(providerName)
		}

		// Display success message
		fmt.Printf("\n%s\n\n", autoDeploySuccessStyle.Render("✓ GitHub Actions workflow created!"))

		fmt.Printf("%s %s\n", autoDeployMutedStyle.Render("Location:"), autoDeployValueStyle.Render(workflowPath))
		fmt.Printf("%s %s\n", autoDeployMutedStyle.Render("Branch:"), autoDeployValueStyle.Render(autoDeployBranch))
		fmt.Printf("%s %s\n", autoDeployMutedStyle.Render("Target:"), autoDeployValueStyle.Render(targetName))
		fmt.Printf("%s %s/%s\n\n", autoDeployMutedStyle.Render("GitHub:"), autoDeployValueStyle.Render(org), autoDeployValueStyle.Render(repo))

		// Display next steps
		fmt.Println(autoDeployLabelStyle.Render("Next steps:"))
		fmt.Println()

		fmt.Println(autoDeployMutedStyle.Render("1. Commit and push the workflow:"))
		fmt.Printf("   git add .github/workflows/lightfold-deploy.yml\n")
		fmt.Printf("   git commit -m \"Add Lightfold auto-deploy workflow\"\n")
		fmt.Printf("   git push\n\n")

		fmt.Println(autoDeployMutedStyle.Render("2. Add GitHub secret via web UI:"))
		githubSecretsURL := fmt.Sprintf("https://github.com/%s/%s/settings/secrets/actions/new", org, repo)
		fmt.Printf("   • Go to: %s\n", autoDeployValueStyle.Render(githubSecretsURL))
		fmt.Printf("   • Name: %s\n", autoDeployValueStyle.Render("PROVIDER_TOKEN"))
		fmt.Printf("   • Value: (copy token below)\n\n")

		if providerToken != "" {
			// Mask token (show last 8 characters)
			maskedToken := "********"
			if len(providerToken) > 8 {
				maskedToken = "********" + providerToken[len(providerToken)-8:]
			}
			fmt.Printf("   %s %s: %s\n", autoDeployMutedStyle.Render("Your"), autoDeployValueStyle.Render(providerName), autoDeployMutedStyle.Render(maskedToken))
			fmt.Printf("   %s %s\n\n", autoDeployMutedStyle.Render("Full token:"), providerToken)
		} else {
			fmt.Printf("   %s No token found for provider '%s'\n", autoDeployErrorStyle.Render("⚠"), providerName)
			fmt.Printf("   Run: lightfold config set-token %s\n\n", providerName)
		}

		fmt.Println(autoDeployMutedStyle.Render("   Alternative (if you have gh CLI installed):"))
		if providerToken != "" {
			fmt.Printf("   gh secret set PROVIDER_TOKEN --body \"%s\"\n\n", providerToken)
		} else {
			fmt.Printf("   gh secret set PROVIDER_TOKEN\n\n")
		}

		fmt.Printf("3. %s\n", autoDeployMutedStyle.Render(fmt.Sprintf("Push to '%s' branch to trigger auto-deploy!", autoDeployBranch)))
		fmt.Println()
	},
}

func init() {
	autoDeployCmd.AddCommand(autoDeploySetupCmd)

	autoDeploySetupCmd.Flags().StringVar(&autoDeployTarget, "target", "", "Target name (defaults to current directory)")
	autoDeploySetupCmd.Flags().StringVar(&autoDeployBranch, "branch", "main", "Git branch to trigger deployment")
	autoDeploySetupCmd.Flags().BoolVar(&autoDeployNoConfirm, "no-confirm", false, "Skip confirmation prompts")
}
