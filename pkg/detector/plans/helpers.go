package plans

import "lightfold/pkg/detector/helpers"

// AddMonorepoMeta adds monorepo metadata if detected
func AddMonorepoMeta(fs FSReader, meta map[string]string) {
	if monorepoType := helpers.DetectMonorepoType(fs); monorepoType != "none" {
		meta["monorepo"] = monorepoType
	}
}
