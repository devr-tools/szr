package main

import (
	"context"
	"os"

	"github.com/devr-tools/szr/internal/szrdev"
)

func main() {
	os.Exit(szrdev.Run(context.Background(), os.Args[1:]))
}
