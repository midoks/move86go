package core

import (
    "os"
    "strconv"
    "strings"
)

func Str2int(s string) int {
    v, _ := strconv.Atoi(strings.TrimSpace(s))
    return v
}

func FileRead(name string) ([]byte, error) {
    return os.ReadFile(name)
}