package main

import (
	"context"
	"os"

	"szr/internal/cli"
)

var version = "dev"

func main() {
	app := cli.New(version)
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}
