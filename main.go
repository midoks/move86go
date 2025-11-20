package main

import (
	"log"
	"os"

	"github.com/urfave/cli"

	"move86go/internal/cmd"
)

const (
	Version = "1.0"
	AppName = "move86go"
)

func main() {
	app := cli.NewApp()
	app.Name = AppName
	app.Version = Version
	app.Usage = "move86go service"
	app.Commands = []cli.Command{
		cmd.Service,
		cmd.Debug,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}
