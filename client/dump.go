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

func dumpRequest(req *http.Request) error {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return nil
	}

	bytes, err := httputil.DumpRequest(req, true)
	if err != nil {
		return err
	}

	fmt.Println("### Dumping HTTP Request ###")
	fmt.Println(string(bytes))
	fmt.Println()

	return nil
}

func dumpResponse(resp *http.Response, forceDump bool) error {
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) && !forceDump {
		return nil
	}

	bytes, err := httputil.DumpResponse(resp, true)
	if err != nil {
		return err
	}

	fmt.Println("### Dumping HTTP Response ###")
	fmt.Println(string(bytes))
	fmt.Println()

	return nil
}
