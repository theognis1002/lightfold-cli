package sequential

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type SSHKeyMode string

const (
	SSHKeyModeFile  SSHKeyMode = "file"
	SSHKeyModePaste SSHKeyMode = "paste"
	SSHKeyModeAuto  SSHKeyMode = "auto"
)

type SSHKeyHandler struct {
	Mode        SSHKeyMode
	Content     string
	FilePath    string
	ProjectName string
	IsMultiline bool
	Buffer      []string
}

func NewSSHKeyHandler(projectName string) *SSHKeyHandler {
	return &SSHKeyHandler{
		Mode:        SSHKeyModeAuto,
		ProjectName: projectName,
		Buffer:      make([]string, 0),
	}
}

func (h *SSHKeyHandler) ProcessInput(input string) error {
	if h.Mode == SSHKeyModeAuto {
		if strings.Contains(input, "BEGIN") && strings.Contains(input, "PRIVATE KEY") {
			h.Mode = SSHKeyModePaste
			h.IsMultiline = true
		} else if strings.HasPrefix(input, "/") || strings.HasPrefix(input, "~") {
			h.Mode = SSHKeyModeFile
		} else {
			h.Mode = SSHKeyModeFile
		}
	}
	switch h.Mode {
	case SSHKeyModeFile:
		return h.processFilePath(input)
	case SSHKeyModePaste:
		return h.processPastedContent(input)
	default:
		return fmt.Errorf("unknown SSH key mode: %s", h.Mode)
	}
}

func (h *SSHKeyHandler) processFilePath(path string) error {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	}

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

func (h *SSHKeyHandler) processPastedContent(content string) error {
	if h.IsMultiline {
		h.Buffer = append(h.Buffer, content)
		fullContent := strings.Join(h.Buffer, "\n")
		if strings.Contains(fullContent, "END") && strings.Contains(fullContent, "PRIVATE KEY") {
			return h.saveContentToFile(fullContent)
		}
		return nil
	}

	if err := h.validateKeyContent(content); err != nil {
		return err
	}

	return h.saveContentToFile(content)
}

func (h *SSHKeyHandler) saveContentToFile(content string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	keysDir := filepath.Join(homeDir, ".lightfold", "keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return fmt.Errorf("cannot create keys directory: %w", err)
	}

	filename := fmt.Sprintf("%s_rsa", h.sanitizeProjectName())
	keyPath := filepath.Join(keysDir, filename)

	if err := os.WriteFile(keyPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("cannot save SSH key: %w", err)
	}

	h.FilePath = keyPath
	h.Content = content
	return nil
}

func (h *SSHKeyHandler) validateKeyContent(content string) error {
	if content == "" {
		return fmt.Errorf("SSH key content is empty")
	}

	hasBegin := strings.Contains(content, "BEGIN") && strings.Contains(content, "PRIVATE KEY")
	hasEnd := strings.Contains(content, "END") && strings.Contains(content, "PRIVATE KEY")

	if !hasBegin {
		return fmt.Errorf("SSH key does not contain valid BEGIN header")
	}

	if !h.IsMultiline && !hasEnd {
		return fmt.Errorf("SSH key does not contain valid END footer")
	}

	return nil
}

func (h *SSHKeyHandler) sanitizeProjectName() string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, h.ProjectName)

	if safe == "" {
		safe = generateRandomString(8)
	}

	return safe
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func (h *SSHKeyHandler) GetFilePath() string {
	return h.FilePath
}

func (h *SSHKeyHandler) GetKeyName() string {
	if h.Mode == SSHKeyModePaste {
		return filepath.Base(h.FilePath)
	}
	return ""
}

func (h *SSHKeyHandler) RenderSSHKeyInput(value string) string {
	var s strings.Builder

	modeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)

	switch h.Mode {
	case SSHKeyModeFile:
		s.WriteString(modeStyle.Render("📁 File Path Mode") + "\n")
	case SSHKeyModePaste:
		s.WriteString(modeStyle.Render("📋 Paste Mode") + "\n")
	default:
		s.WriteString(modeStyle.Render("🔑 SSH Key Input") + "\n")
	}

	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#01FAC6")).
		Padding(0, 1).
		Width(60)

	displayValue := value
	if h.IsMultiline && len(h.Buffer) > 0 {
		displayValue = fmt.Sprintf("[%d lines pasted...]", len(h.Buffer))
		if value != "" {
			displayValue += "\n" + value
		}
	}

	if displayValue == "" {
		displayValue = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render("Enter file path or paste SSH key")
	}

	displayValue += "│"

	s.WriteString(inputStyle.Render(displayValue))
	s.WriteString("\n\n")

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.WriteString(helpStyle.Render("• Enter file path (e.g., ~/.ssh/id_rsa)"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("• Or paste SSH private key content"))
	s.WriteString("\n")
	s.WriteString(helpStyle.Render("• Pasted keys will be saved to ~/.lightfold/keys/"))

	return s.String()
}

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
