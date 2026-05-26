package workflows

import (
	"io"

	"github.com/devr-tools/szr/internal/config"
	"github.com/devr-tools/szr/internal/history"
)

type Runtime struct {
	Config                config.Config
	Paths                 config.Paths
	History               *history.Store
	Stdout                io.Writer
	Stderr                io.Writer
	Verbose               int
	UltraCompact          bool
	DescribeProfileSource func(source string, projectRuleFile string) string
	RelativeToRepo        func(root, path string) string
}
