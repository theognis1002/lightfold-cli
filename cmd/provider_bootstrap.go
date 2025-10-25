package cmd

import (
	"fmt"
	"lightfold/cmd/ui/sequential"
	"lightfold/pkg/config"
	sshpkg "lightfold/pkg/ssh"
	"os"
	"path/filepath"
	"strings"
)

type providerBootstrap struct {
	canonical        string
	aliases          []string
	configKey        string
	tokenKey         string
	defaultUsername  string
	fallbackFlow     func(targetName string) (config.ProviderConfig, error)
	flagConfigurator func(opts provisionInputs, sshKeyPath string, sshKeyName string) (config.ProviderConfig, error)
}

type provisionInputs struct {
	Region string
	Size   string
	Image  string
}

var providerBootstraps = []*providerBootstrap{
	{
		canonical:       "digitalocean",
		aliases:         []string{"digitalocean", "do"},
		configKey:       "digitalocean",
		tokenKey:        "digitalocean",
		defaultUsername: "root",
		fallbackFlow: func(targetName string) (config.ProviderConfig, error) {
			cfg, err := sequential.RunProvisionDigitalOceanFlow(targetName)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		},
		flagConfigurator: func(opts provisionInputs, sshKeyPath, sshKeyName string) (config.ProviderConfig, error) {
			if opts.Region == "" {
				return nil, fmt.Errorf("region is required for DigitalOcean provisioning")
			}
			if opts.Size == "" {
				return nil, fmt.Errorf("size is required for DigitalOcean provisioning")
			}
			return &config.DigitalOceanConfig{
				Region:      opts.Region,
				Size:        opts.Size,
				SSHKey:      sshKeyPath,
				SSHKeyName:  sshKeyName,
				Username:    "deploy",
				Provisioned: true,
			}, nil
		},
	},
	{
		canonical:       "hetzner",
		aliases:         []string{"hetzner"},
		configKey:       "hetzner",
		tokenKey:        "hetzner",
		defaultUsername: "deploy",
		fallbackFlow: func(targetName string) (config.ProviderConfig, error) {
			cfg, err := sequential.RunProvisionHetznerFlow(targetName)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		},
		flagConfigurator: func(opts provisionInputs, sshKeyPath, sshKeyName string) (config.ProviderConfig, error) {
			if opts.Region == "" {
				return nil, fmt.Errorf("location is required for Hetzner provisioning")
			}
			if opts.Size == "" {
				return nil, fmt.Errorf("server type is required for Hetzner provisioning")
			}
			return &config.HetznerConfig{
				Location:    opts.Region,
				ServerType:  opts.Size,
				SSHKey:      sshKeyPath,
				SSHKeyName:  sshKeyName,
				Username:    "deploy",
				Provisioned: true,
			}, nil
		},
	},
	{
		canonical:       "vultr",
		aliases:         []string{"vultr"},
		configKey:       "vultr",
		tokenKey:        "vultr",
		defaultUsername: "deploy",
		fallbackFlow: func(targetName string) (config.ProviderConfig, error) {
			cfg, err := sequential.RunProvisionVultrFlow(targetName)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		},
		flagConfigurator: func(opts provisionInputs, sshKeyPath, sshKeyName string) (config.ProviderConfig, error) {
			if opts.Region == "" {
				return nil, fmt.Errorf("region is required for Vultr provisioning")
			}
			if opts.Size == "" {
				return nil, fmt.Errorf("plan is required for Vultr provisioning")
			}
			return &config.VultrConfig{
				Region:      opts.Region,
				Plan:        opts.Size,
				SSHKey:      sshKeyPath,
				SSHKeyName:  sshKeyName,
				Username:    "deploy",
				Provisioned: true,
			}, nil
		},
	},
	{
		canonical:       "flyio",
		aliases:         []string{"flyio"},
		configKey:       "flyio",
		tokenKey:        "flyio",
		defaultUsername: "deploy",
		fallbackFlow: func(targetName string) (config.ProviderConfig, error) {
			cfg, err := sequential.RunProvisionFlyioFlow(targetName)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		},
		flagConfigurator: func(opts provisionInputs, sshKeyPath, sshKeyName string) (config.ProviderConfig, error) {
			if opts.Region == "" {
				return nil, fmt.Errorf("region is required for fly.io provisioning")
			}
			if opts.Size == "" {
				return nil, fmt.Errorf("size is required for fly.io provisioning")
			}
			return &config.FlyioConfig{
				Region:      opts.Region,
				Size:        opts.Size,
				SSHKey:      sshKeyPath,
				SSHKeyName:  sshKeyName,
				Username:    "root",
				Provisioned: true,
			}, nil
		},
	},
	{
		canonical:       "linode",
		aliases:         []string{"linode"},
		configKey:       "linode",
		tokenKey:        "linode",
		defaultUsername: "deploy",
		fallbackFlow: func(targetName string) (config.ProviderConfig, error) {
			cfg, err := sequential.RunProvisionLinodeFlow(targetName)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		},
		flagConfigurator: func(opts provisionInputs, sshKeyPath, sshKeyName string) (config.ProviderConfig, error) {
			if opts.Region == "" {
				return nil, fmt.Errorf("region is required for Linode provisioning")
			}
			if opts.Size == "" {
				return nil, fmt.Errorf("plan is required for Linode provisioning")
			}
			return &config.LinodeConfig{
				Region:      opts.Region,
				Plan:        opts.Size,
				SSHKey:      sshKeyPath,
				SSHKeyName:  sshKeyName,
				Username:    "deploy",
				Provisioned: true,
			}, nil
		},
	},
	{
		canonical:       "aws",
		aliases:         []string{"aws", "ec2"},
		configKey:       "aws",
		tokenKey:        "aws",
		defaultUsername: "ubuntu",
		fallbackFlow: func(targetName string) (config.ProviderConfig, error) {
			cfg, err := sequential.RunProvisionAWSFlow(targetName)
			if err != nil {
				return nil, err
			}
			return cfg, nil
		},
		flagConfigurator: func(opts provisionInputs, sshKeyPath, sshKeyName string) (config.ProviderConfig, error) {
			if opts.Region == "" {
				return nil, fmt.Errorf("region is required for AWS provisioning")
			}
			if opts.Size == "" {
				return nil, fmt.Errorf("instance type is required for AWS provisioning")
			}
			return &config.AWSConfig{
				Region:       opts.Region,
				InstanceType: opts.Size,
				SSHKey:       sshKeyPath,
				SSHKeyName:   sshKeyName,
				Username:     "ubuntu",
				Provisioned:  true,
			}, nil
		},
	},
}

var providerAliasMap map[string]*providerBootstrap

func init() {
	providerAliasMap = make(map[string]*providerBootstrap)
	for _, spec := range providerBootstraps {
		for _, alias := range spec.aliases {
			providerAliasMap[strings.ToLower(alias)] = spec
		}
	}
}

func findProviderBootstrap(name string) (*providerBootstrap, error) {
	spec, ok := providerAliasMap[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", name)
	}
	return spec, nil
}

func ensureProvisionSSHKey(username string) (path string, keyName string, err error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	keyDir := filepath.Join(homeDir, config.LocalConfigDir, config.LocalKeysDir)
	if err := os.MkdirAll(keyDir, 0o755); err != nil && !os.IsExist(err) {
		return "", "", fmt.Errorf("failed to ensure key directory: %w", err)
	}

	keyPath := filepath.Join(keyDir, "lightfold_ed25519")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		publicKeyPath, genErr := sshpkg.GenerateKeyPair(keyPath)
		if genErr != nil {
			return "", "", fmt.Errorf("failed to generate SSH key pair: %w", genErr)
		}
		_ = publicKeyPath
	}

	return keyPath, filepath.Base(keyPath), nil
}

func (p *providerBootstrap) applyConfig(targetConfig *config.TargetConfig, providerConfig interface{}) error {
	switch cfg := providerConfig.(type) {
	case *config.DigitalOceanConfig:
		targetConfig.Provider = p.canonical
		return targetConfig.SetProviderConfig(p.configKey, cfg)
	case *config.HetznerConfig:
		targetConfig.Provider = p.canonical
		return targetConfig.SetProviderConfig(p.configKey, cfg)
	case *config.VultrConfig:
		targetConfig.Provider = p.canonical
		return targetConfig.SetProviderConfig(p.configKey, cfg)
	case *config.FlyioConfig:
		targetConfig.Provider = p.canonical
		return targetConfig.SetProviderConfig(p.configKey, cfg)
	case *config.LinodeConfig:
		targetConfig.Provider = p.canonical
		return targetConfig.SetProviderConfig(p.configKey, cfg)
	case *config.AWSConfig:
		targetConfig.Provider = p.canonical
		return targetConfig.SetProviderConfig(p.configKey, cfg)
	default:
		return fmt.Errorf("unexpected provider configuration type for %s", p.canonical)
	}
}

func (p *providerBootstrap) prepareConfigFromFlags(targetName string, opts provisionInputs) (config.ProviderConfig, error) {
	sshPath, sshName, err := ensureProvisionSSHKey(p.defaultUsername)
	if err != nil {
		return nil, err
	}

	if p.flagConfigurator == nil {
		return nil, fmt.Errorf("provider %s does not support flag-based provisioning", p.canonical)
	}

	cfg, err := p.flagConfigurator(opts, sshPath, sshName)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (p *providerBootstrap) ensureToken(targetName string) (string, config.ProviderConfig, error) {
	tokens, err := config.LoadTokens()
	if err != nil {
		return "", nil, fmt.Errorf("failed to load tokens: %w", err)
	}
	if tokens == nil {
		return "", nil, fmt.Errorf("token store not initialised")
	}

	token := tokens.GetToken(p.tokenKey)
	if token != "" {
		return token, nil, nil
	}

	if p.fallbackFlow == nil {
		return "", nil, fmt.Errorf("no token available for provider %s", p.canonical)
	}

	cfg, err := p.fallbackFlow(targetName)
	if err != nil {
		return "", nil, err
	}

	tokens, err = config.LoadTokens()
	if err != nil {
		return "", nil, fmt.Errorf("failed to reload tokens: %w", err)
	}

	token = tokens.GetToken(p.tokenKey)
	if token == "" {
		return "", cfg, fmt.Errorf("no %s API token provided", p.canonical)
	}

	return token, cfg, nil
}
