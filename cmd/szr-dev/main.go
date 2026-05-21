package main

import (
	"context"
	"os"

	"szr/internal/szrdev"
)

func main() {
	os.Exit(szrdev.Run(context.Background(), os.Args[1:]))
}
