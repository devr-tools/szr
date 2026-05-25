package main

import (
	"context"
	"os"

	szrpkg "github.com/devr-tools/szr/pkg/szr"
)

var version = "dev"

func main() {
	os.Exit(szrpkg.Run(context.Background(), version, os.Args[1:]))
}
