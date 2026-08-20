package cli

import (
	"bufio"
	"io"

	"github.com/devr-tools/szr/internal/config"
)

func settingsProjectFiltersAction(app *App, reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	return app.updateBooleanSetting(reader, stdout, stderr, "project filters", app.config.Advanced.ProjectFilters, "settings: project filters unchanged", func(cfg *config.Config, value bool) string {
		cfg.Advanced.ProjectFilters = value
		return enabledLabel(cfg.Advanced.ProjectFilters)
	})
}

func settingsProjectRulesAction(app *App, reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	return app.updateBooleanSetting(reader, stdout, stderr, "project rules", app.config.Advanced.ProjectRules, "settings: project rules unchanged", func(cfg *config.Config, value bool) string {
		cfg.Advanced.ProjectRules = value
		return enabledLabel(cfg.Advanced.ProjectRules)
	})
}
