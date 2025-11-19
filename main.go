package main

import (
    "os"
    "os/signal"
    
    "move86go/core"
    "move86go/core/lagran"
    "move86go/core/logx"
)

func main() {
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
}
