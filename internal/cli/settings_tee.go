package cli

import (
	"bufio"
	"fmt"
	"io"

	"github.com/devr-tools/szr/internal/config"
)

func settingsTeeMaxFileAction(app *App, reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	return app.updatePositiveIntSetting(reader, stdout, stderr, "tee max file mb", "tee max file mb", "settings: tee max file mb unchanged", func(cfg *config.Config, value int) string {
		cfg.TeeMaxFileMB = value
		return fmt.Sprintf("%dMB", cfg.TeeMaxFileMB)
	})
}

func settingsTeeMaxDirFilesAction(app *App, reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	return app.updatePositiveIntSetting(reader, stdout, stderr, "tee max dir files", "tee max dir files", "settings: tee max dir files unchanged", func(cfg *config.Config, value int) string {
		cfg.TeeMaxDirFiles = value
		return fmt.Sprintf("%d", cfg.TeeMaxDirFiles)
	})
}

func settingsTeeMaxDirSizeAction(app *App, reader *bufio.Reader, stdout, stderr io.Writer) (int, bool) {
	return app.updatePositiveIntSetting(reader, stdout, stderr, "tee max dir mb", "tee max dir mb", "settings: tee max dir mb unchanged", func(cfg *config.Config, value int) string {
		cfg.TeeMaxDirMB = value
		return fmt.Sprintf("%dMB", cfg.TeeMaxDirMB)
	})
}
