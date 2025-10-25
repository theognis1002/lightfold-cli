package providers

// ProviderImageDefaults maps provider names to their default OS images.
// These are Ubuntu 22.04 LTS equivalents for each provider.
// Update this when new LTS versions are released (next: Ubuntu 24.04).
var ProviderImageDefaults = map[string]string{
	"digitalocean": "ubuntu-22-04-x64",
	"hetzner":      "ubuntu-22.04",
	"vultr":        "1743",               // Ubuntu 22.04 LTS x64 - Vultr uses numeric IDs
	"flyio":        "ubuntu:22.04",       // Docker image format
	"linode":       "linode/ubuntu22.04", // Linode image format
	"aws":          "ubuntu-22.04",       // Placeholder - actual AMI resolved per region at runtime
}

// GetDefaultImage returns the default OS image for the given provider.
// Falls back to Ubuntu 22.04 generic identifier if provider not found.
func GetDefaultImage(provider string) string {
	if image, ok := ProviderImageDefaults[provider]; ok {
		return image
	}
	// Generic fallback for unknown providers
	return "ubuntu-22.04"
}
