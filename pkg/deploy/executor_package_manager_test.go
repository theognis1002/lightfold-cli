package deploy

import "testing"

func TestAdjustPackageManagerPath(t *testing.T) {
	tests := []struct {
		name           string
		runCommand     string
		packageManager string
		expected       string
	}{
		{
			name:           "bun run start",
			runCommand:     "bun run start",
			packageManager: "bun",
			expected:       "/home/deploy/.bun/bin/bun run start",
		},
		{
			name:           "bun start (no run)",
			runCommand:     "bun start",
			packageManager: "bun",
			expected:       "/home/deploy/.bun/bin/bun start",
		},
		{
			name:           "npm start",
			runCommand:     "npm start",
			packageManager: "npm",
			expected:       "/usr/bin/npm start",
		},
		{
			name:           "npm run dev",
			runCommand:     "npm run dev",
			packageManager: "npm",
			expected:       "/usr/bin/npm run dev",
		},
		{
			name:           "pnpm start",
			runCommand:     "pnpm start",
			packageManager: "pnpm",
			expected:       "/home/deploy/.local/share/pnpm/pnpm start",
		},
		{
			name:           "yarn start",
			runCommand:     "yarn start",
			packageManager: "yarn",
			expected:       "/usr/bin/yarn start",
		},
		{
			name:           "unknown package manager",
			runCommand:     "deno run main.ts",
			packageManager: "deno",
			expected:       "deno run main.ts",
		},
		{
			name:           "empty package manager",
			runCommand:     "node server.js",
			packageManager: "",
			expected:       "node server.js",
		},
		{
			name:           "bun in middle of command (should NOT replace)",
			runCommand:     "echo bun is great",
			packageManager: "bun",
			expected:       "echo bun is great",
		},
		{
			name:           "command doesn't start with package manager",
			runCommand:     "/usr/bin/node server.js",
			packageManager: "npm",
			expected:       "/usr/bin/node server.js",
		},
		{
			name:           "npm with multiple spaces",
			runCommand:     "npm  start",
			packageManager: "npm",
			expected:       "/usr/bin/npm  start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustPackageManagerPath(tt.runCommand, tt.packageManager)
			if result != tt.expected {
				t.Errorf("adjustPackageManagerPath(%q, %q) = %q; want %q",
					tt.runCommand, tt.packageManager, result, tt.expected)
			}
		})
	}
}
