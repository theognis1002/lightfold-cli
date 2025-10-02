// Package steps provides utility for creating
// each step of the CLI flow
package steps

import "lightfold/cmd/flags"

// A StepSchema contains the data that is used
// for an individual step of the CLI
type StepSchema struct {
	StepName string // The name of a given step
	Options  []Item // The slice of each option for a given step
	Headers  string // The title displayed at the top of a given step
	Field    string
}

// Steps contains a map of steps
type Steps struct {
	Steps map[string]StepSchema
}

// An Item contains the data for each option
// in a StepSchema.Options
type Item struct {
	Flag, Title, Desc string
}

// InitSteps initializes and returns the *Steps to be used in the CLI program
func InitSteps(targetType flags.DeploymentTarget) *Steps {
	steps := &Steps{
		map[string]StepSchema{
			"target": {
				StepName: "Deployment Target",
				Options: []Item{
					{
						Title: "DigitalOcean",
						Desc:  "Deploy to a DigitalOcean droplet via SSH",
					},
					{
						Title: "S3",
						Desc:  "Deploy static sites to AWS S3 bucket",
					},
				},
				Headers: "What deployment target would you like?",
				Field:   targetType.String(),
			},
		},
	}

	return steps
}