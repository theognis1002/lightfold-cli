package cmd

import (
	"lightfold/cmd/utils"
	"lightfold/pkg/config"
	"lightfold/pkg/state"
)

type providerStateHandler struct {
	displayName string
	cfgAccessor func(*config.TargetConfig) (config.ProviderConfig, error)
	recoverFunc func(*config.TargetConfig, string, string) error
}

var providerStateHandlers = map[string]providerStateHandler{
	"digitalocean": {
		displayName: "DigitalOcean",
		cfgAccessor: func(target *config.TargetConfig) (config.ProviderConfig, error) {
			return target.GetDigitalOceanConfig()
		},
		recoverFunc: func(target *config.TargetConfig, targetName, serverID string) error {
			return utils.RecoverIPFromProvider(target, targetName, "digitalocean", serverID)
		},
	},
	"hetzner": {
		displayName: "Hetzner",
		cfgAccessor: func(target *config.TargetConfig) (config.ProviderConfig, error) {
			return target.GetHetznerConfig()
		},
		recoverFunc: func(target *config.TargetConfig, targetName, serverID string) error {
			return utils.RecoverIPFromProvider(target, targetName, "hetzner", serverID)
		},
	},
	"vultr": {
		displayName: "Vultr",
		cfgAccessor: func(target *config.TargetConfig) (config.ProviderConfig, error) {
			return target.GetVultrConfig()
		},
		recoverFunc: func(target *config.TargetConfig, targetName, serverID string) error {
			return utils.RecoverIPFromProvider(target, targetName, "vultr", serverID)
		},
	},
	"flyio": {
		displayName: "fly.io",
		cfgAccessor: func(target *config.TargetConfig) (config.ProviderConfig, error) {
			return target.GetFlyioConfig()
		},
		recoverFunc: func(target *config.TargetConfig, targetName, serverID string) error {
			return utils.RecoverIPFromFlyio(target, targetName, serverID)
		},
	},
	"linode": {
		displayName: "Linode",
		cfgAccessor: func(target *config.TargetConfig) (config.ProviderConfig, error) {
			return target.GetLinodeConfig()
		},
		recoverFunc: func(target *config.TargetConfig, targetName, serverID string) error {
			return utils.RecoverIPFromProvider(target, targetName, "linode", serverID)
		},
	},
}

func tryRecoverProviderIP(target *config.TargetConfig, targetName string, targetState *state.TargetState) (bool, string, error) {
	handler, ok := providerStateHandlers[target.Provider]
	if !ok {
		return false, "", nil
	}

	providerCfg, err := handler.cfgAccessor(target)
	if err != nil || providerCfg == nil {
		return false, handler.displayName, nil
	}

	if !providerCfg.IsProvisioned() {
		return false, handler.displayName, nil
	}

	serverID := providerCfg.GetServerID()
	if serverID == "" && targetState != nil {
		serverID = targetState.ProvisionedID
	}

	if serverID == "" {
		if cachedState, err := state.LoadState(targetName); err == nil {
			targetState = cachedState
			serverID = cachedState.ProvisionedID
		}
	}

	if providerCfg.GetIP() != "" || serverID == "" {
		return false, handler.displayName, nil
	}

	if err := handler.recoverFunc(target, targetName, serverID); err != nil {
		return false, handler.displayName, err
	}

	return true, handler.displayName, nil
}
