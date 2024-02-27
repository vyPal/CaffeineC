package main

import (
	"os"

	"github.com/urfave/cli/v2"
)

var commands []*cli.Command

func main() {
	app := &cli.App{
		Name:                   "CaffeineC",
		Usage:                  "A C-like language that compiles to LLVM IR",
		EnableBashCompletion:   true,
		Suggest:                true,
		UseShortOptionHandling: true,
		Version:                "2.2.12",
		Commands:               commands,
	}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}
