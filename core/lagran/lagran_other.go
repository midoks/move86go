//go:build !linux
package lagran

import (
    "runtime"
    "move86go/core/logx"
)

var HttpPort = "80,443,8888"

func Run() {
    logx.Error("[lagran] unsupported os:", runtime.GOOS)
}

func UnsetIptable(sport string) {}