package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
)

func dumpBody(bytes []byte) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	fmt.Println("### Dumping HTTP Request Body ###")
	fmt.Println(string(bytes))
	fmt.Println()
}

func dumpRequest(logger *slog.Logger, req *http.Request) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	bytes, err := httputil.DumpRequest(req, true)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	fmt.Println("### Dumping HTTP Request ###")
	fmt.Println(string(bytes))
	fmt.Println()
}

func dumpResponse(logger *slog.Logger, resp *http.Response) {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	bytes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		logger.Error(err.Error())
		panic(err)
	}

	fmt.Println("### Dumping HTTP Response ###")
	fmt.Println(string(bytes))
	fmt.Println()
}
