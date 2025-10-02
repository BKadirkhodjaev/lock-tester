package util

import (
	"errors"
	"log/slog"
	"runtime"
)

func LogErrorPanic(commandName string, errorMessage string) {
	slog.Error(commandName, errorMessage, "")
	panic(errors.New(errorMessage))
}

func GetFuncName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}
