//go:build !windows

package engine

import (
	"errors"
	"syscall"
)

func isExecTextFileBusy(err error) bool {
	return errors.Is(err, syscall.ETXTBSY)
}
