package flags

import (
	"fmt"
	"strings"
)

type DeploymentTarget string

// Deployment targets supported by lightfold
const (
	DigitalOcean DeploymentTarget = "digitalocean"
	S3           DeploymentTarget = "s3"
)

var AllowedTargets = []string{string(DigitalOcean), string(S3)}

func (t DeploymentTarget) String() string {
	return string(t)
}

func (t *DeploymentTarget) Type() string {
	return "DeploymentTarget"
}

func (t *DeploymentTarget) Set(value string) error {
	for _, target := range AllowedTargets {
		if target == value {
			*t = DeploymentTarget(value)
			return nil
		}
	}

	return fmt.Errorf("Deployment target. Allowed values: %s", strings.Join(AllowedTargets, ", "))
}