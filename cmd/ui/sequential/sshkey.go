package sequential

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// SSHKeyMode represents the different ways to handle SSH keys
type SSHKeyMode string

const (
	SSHKeyModeFile  SSHKeyMode = "file"    // Use existing file path
	SSHKeyModePaste SSHKeyMode = "paste"   // Paste key content
	SSHKeyModeAuto  SSHKeyMode = "auto"    // Auto-detect based on input
)

// SSHKeyHandler manages SSH key input and storage
type SSHKeyHandler struct {
	Mode        SSHKeyMode
	Content     string
	FilePath    string
	ProjectName string
	IsMultiline bool
	Buffer      []string // For multiline paste input
}

// NewSSHKeyHandler creates a new SSH key handler
func NewSSHKeyHandler(projectName string) *SSHKeyHandler {
	return &SSHKeyHandler{
		Mode:        SSHKeyModeAuto,
		ProjectName: projectName,
		Buffer:      make([]string, 0),
	}
}

// ProcessInput processes SSH key input and determines the appropriate mode
func (h *SSHKeyHandler) ProcessInput(input string) error {
	// Auto-detect mode based on input
	if h.Mode == SSHKeyModeAuto {
		if strings.Contains(input, "BEGIN") && strings.Contains(input, "PRIVATE KEY") {
			h.Mode = SSHKeyModePaste
			h.IsMultiline = true
		} else if strings.HasPrefix(input, "/") || strings.HasPrefix(input, "~") {
			h.Mode = SSHKeyModeFile
		} else {
			// Default to file mode for relative paths or simple names
			h.Mode = SSHKeyModeFile
		}
	}

	// Process based on mode
	switch h.Mode {
	case SSHKeyModeFile:
		return h.processFilePath(input)
	case SSHKeyModePaste:
		return h.processPastedContent(input)
	default:
		return fmt.Errorf("unknown SSH key mode: %s", h.Mode)
	}
}

// processFilePath handles file path input
func (h *SSHKeyHandler) processFilePath(path string) error {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

	// Validate file exists and is readable
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("SSH key file does not exist: %s", path)
	}

	// Basic validation of key content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read SSH key file: %w", err)
	}

	if err := h.validateKeyContent(string(content)); err != nil {
		return err
	}

	h.FilePath = path
	h.Content = string(content)
	return nil
}

// processPastedContent handles pasted SSH key content
func (h *SSHKeyHandler) processPastedContent(content string) error {
	// For multiline input, we might be building up the content
	if h.IsMultiline {
		h.Buffer = append(h.Buffer, content)
		// Check if we have a complete key
		fullContent := strings.Join(h.Buffer, "\n")
		if strings.Contains(fullContent, "END") && strings.Contains(fullContent, "PRIVATE KEY") {
			return h.saveContentToFile(fullContent)
		}
		// Still building the key
		return nil
	}

	// Single-line or complete content
	if err := h.validateKeyContent(content); err != nil {
		return err
	}

	return h.saveContentToFile(content)
}

// saveContentToFile saves pasted SSH key content to a project-specific file
func (h *SSHKeyHandler) saveContentToFile(content string) error {
	// Create lightfold keys directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	keysDir := filepath.Join(homeDir, ".lightfold", "keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("cannot create keys directory: %w", err)
	}

	// Generate unique filename for this project
	filename := fmt.Sprintf("%s_rsa", h.sanitizeProjectName())
	keyPath := filepath.Join(keysDir, filename)

	// Write the key content
	if err := os.WriteFile(keyPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("cannot save SSH key: %w", err)
	}

	h.FilePath = keyPath
	h.Content = content
	return nil
}

// validateKeyContent validates SSH key content format
func (h *SSHKeyHandler) validateKeyContent(content string) error {
	if content == "" {
		return fmt.Errorf("SSH key content is empty")
	}

	// Check for private key headers
	hasBegin := strings.Contains(content, "BEGIN") && strings.Contains(content, "PRIVATE KEY")
	hasEnd := strings.Contains(content, "END") && strings.Contains(content, "PRIVATE KEY")

	if !hasBegin {
		return fmt.Errorf("SSH key does not contain valid BEGIN header")
	}

	// For multiline input, we might not have the END yet
	if !h.IsMultiline && !hasEnd {
		return fmt.Errorf("SSH key does not contain valid END footer")
	}

	return nil
}

// sanitizeProjectName creates a safe filename from project name
func (h *SSHKeyHandler) sanitizeProjectName() string {
	// Remove invalid filename characters
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, h.ProjectName)

	// Ensure it's not empty
	if safe == "" {
		safe = generateRandomString(8)
	}

	return safe
}

// generateRandomString generates a random string for fallback naming
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// GetFilePath returns the final file path for the SSH key
func (h *SSHKeyHandler) GetFilePath() string {
	return h.FilePath
}

// GetKeyName returns the managed key name (for config storage)
func (h *SSHKeyHandler) GetKeyName() string {
	if h.Mode == SSHKeyModePaste {
		return filepath.Base(h.FilePath)
	}
	return ""
}

// RenderSSHKeyInput renders the SSH key input interface
func (h *SSHKeyHandler) RenderSSHKeyInput(value string) string {
	var s strings.Builder

	// Input mode indicator
	modeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)

	switch h.Mode {
	case SSHKeyModeFile:
		s.WriteString(modeStyle.Render("📁 File Path Mode") + "\n")
	case SSHKeyModePaste:
		s.WriteString(modeStyle.Render("📋 Paste Mode") + "\n")
	default:
		s.WriteString(modeStyle.Render("🔑 SSH Key Input") + "\n")
	}

	// Input area
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#01FAC6")).
		Padding(0, 1).
		Width(60)

	displayValue := value
	if h.IsMultiline && len(h.Buffer) > 0 {
		// Show multiline progress
		displayValue = fmt.Sprintf("[%d lines pasted...]", len(h.Buffer))
		if value != "" {
			displayValue += "\n" + value
		}
	}

	if displayValue == "" {
		displayValue = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render("Enter file path or paste SSH key")
	}

	// Add cursor
	displayValue += "│"

	s.WriteString(inputStyle.Render(displayValue))
	s.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.WriteString(helpStyle.Render("• Enter file path (e.g., ~/.ssh/id_rsa)"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("• Or paste SSH private key content"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("• Pasted keys will be saved to ~/.lightfold/keys/"))

	return s.String()
}

// GetStatus returns the current status of the SSH key handler
func (h *SSHKeyHandler) GetStatus() string {
	switch h.Mode {
	case SSHKeyModeFile:
		if h.FilePath != "" {
			return fmt.Sprintf("Using SSH key: %s", h.FilePath)
		}
		return "Enter SSH key file path"
	case SSHKeyModePaste:
		if h.FilePath != "" {
			return fmt.Sprintf("SSH key saved to: %s", h.FilePath)
		}
		if h.IsMultiline && len(h.Buffer) > 0 {
			return fmt.Sprintf("Building SSH key... (%d lines)", len(h.Buffer))
		}
		return "Paste SSH key content"
	default:
		return "SSH key input"
	}
}