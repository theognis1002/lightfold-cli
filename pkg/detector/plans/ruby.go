package plans

import (
	"lightfold/pkg/config"
)

// RailsPlan returns the build and run plan for Rails
func RailsPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"bundle install --deployment --without development test",
		"bundle exec rails db:migrate",
		"bundle exec rails assets:precompile",
	}
	run := []string{
		"bundle exec puma -C config/puma.rb",
	}
	health := map[string]any{"path": "/up", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"RAILS_ENV", "DATABASE_URL", "SECRET_KEY_BASE"}
	meta := map[string]string{}
	return build, run, health, env, meta
}

// JekyllPlan returns the build and run plan for Jekyll
func JekyllPlan(root string) ([]string, []string, map[string]any, []string, map[string]string) {
	build := []string{
		"bundle install",
		"bundle exec jekyll build",
	}
	run := []string{
		"bundle exec jekyll serve --host 0.0.0.0",
	}
	health := map[string]any{"path": "/", "expect": config.DefaultHealthCheckStatus, "timeout_seconds": int(config.DefaultHealthCheckTimeout.Seconds())}
	env := []string{"JEKYLL_ENV"}
	meta := map[string]string{"build_output": "_site/", "static": "true"}
	return build, run, health, env, meta
}
