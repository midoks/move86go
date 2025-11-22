//go:build !linux

package lagran

import (
	"move86go/core/logx"
	"runtime"
)

var HttpPort = "80,443,8888"

var M86Debug = false

func SetDebug(d bool) {
	M86Debug = d
}

func Run() {
	logx.Error("[lagran] unsupported os:", runtime.GOOS)
}

func UnsetIptable(sport string) {}
