//go:build !linux

package lagran

import (
	"move86go/core/logx"
	"runtime"
)

var HttpPort = "80,443,8888"

func Run() {
	logx.Error("[lagran] unsupported os:", runtime.GOOS)
}

func UnsetIptable(sport string) {}
