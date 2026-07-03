package cli

import (
	"os"

	localcmd "github.com/devr-tools/szr/internal/cli/localcmd"
	"github.com/devr-tools/szr/internal/config"
)

// runPipe summarizes stdin that is already flowing through a pipe
// (`<cmd> | szr pipe`). The read end of a pipe cannot observe the producer's
// exit status, so the render never claims success or failure; the returned
// exit code covers only szr's own filtering work.
func (a *App) runPipe(cfg config.Config, args []string) int {
	return localcmd.RunPipe(localRuntime(a), cfg, args, os.Stdin, isInteractiveFile(os.Stdin))
}
