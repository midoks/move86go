package cmd

import (
	"os"
	"os/signal"

	"github.com/urfave/cli"

	"move86go/core"
	"move86go/core/lagran"
	"move86go/core/logx"
)

var Service = cli.Command{
	Name:        "service",
	Usage:       "this command start service",
	Description: `start service`,
	Action:      runService,
	Flags: []cli.Flag{
		stringFlag("config, c", "", "custom configuration file path"),
	},
}

func runService(c *cli.Context) error {
	logx.Info("Run")
	if portData, err := core.FileRead("port.txt"); err == nil {
		lagran.HttpPort = string(portData)
	}
	go lagran.Run()
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, os.Interrupt)
	go func() {
		<-sigs
		lagran.UnsetIptable(lagran.HttpPort)
		done <- true
	}()
	<-done
	logx.Info("exit")

	return nil
}
