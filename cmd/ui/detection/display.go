package detection

import (
	"lightfold/cmd/ui/deployment"
	"lightfold/pkg/detector"
)

// ShowDetectionResults displays the detection results and deployment editor
// Returns: (wantsDeploy, buildCommands, runCommands, error)
func ShowDetectionResults(detection detector.Detection) (bool, []string, []string, error) {
	return deployment.ShowDeploymentEditor(detection, nil, nil)
}
