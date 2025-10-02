package steps

import "lightfold/cmd/flags"

type StepSchema struct {
	StepName string
	Options  []Item
	Headers  string
	Field    string
}

type Steps struct {
	Steps map[string]StepSchema
}

type Item struct {
	Flag, Title, Desc string
}

func InitSteps(targetType flags.DeploymentTarget) *Steps {
	steps := &Steps{
		map[string]StepSchema{
			"deployment_type": {
				StepName: "Deployment Type",
				Options: []Item{
					{
						Title: "BYOS",
						Desc:  "Bring Your Own Server - deploy to existing infrastructure",
					},
					{
						Title: "Provision for me",
						Desc:  "Auto-provision new cloud infrastructure",
					},
				},
				Headers: "How would you like to deploy?",
				Field:   targetType.String(),
			},
			"byos_target": {
				StepName: "BYOS Target",
				Options: []Item{
					{
						Title: "DigitalOcean",
						Desc:  "Deploy to existing DigitalOcean droplet via SSH",
					},
					{
						Title: "Custom Server",
						Desc:  "Deploy to any server with SSH access",
					},
				},
				Headers: "Which server provider?",
				Field:   targetType.String(),
			},
			"provision_target": {
				StepName: "Provision Target",
				Options: []Item{
					{
						Title: "DigitalOcean",
						Desc:  "Auto-provision new DigitalOcean droplet",
					},
					{
						Title: "S3",
						Desc:  "Deploy static sites to AWS S3 bucket",
					},
				},
				Headers: "Where would you like to provision?",
				Field:   targetType.String(),
			},
		},
	}

	return steps
}