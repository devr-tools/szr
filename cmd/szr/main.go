package main

import (
	"context"
	"os"

	szrpkg "szr/pkg/szr"
)

var version = "dev"

func main() {
	os.Exit(szrpkg.Run(context.Background(), version, os.Args[1:]))
}
