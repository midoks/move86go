package logx

import (
	"fmt"
	"os"
	"time"
)

var IsDebug = true

func Info(a ...any) {
	fmt.Println("[Info][", time.Now().Format("2006-01-02 15:04:05"), "]:", a)
}

func Debug(a ...any) {
	if IsDebug {
		fmt.Println("[Debug][", time.Now().Format("2006-01-02 15:04:05"), "]:", a)
	}
}

func Error(a ...any) {
	fmt.Fprintln(os.Stderr, "[Error][", time.Now().Format("2006-01-02 15:04:05"), "]:", a)
}
