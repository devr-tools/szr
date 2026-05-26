package localcmd

import "io"

type Runtime struct {
	Stdout io.Writer
	Stderr io.Writer
}
