# Lock tester

## Purpose

- Command line utility that performs parallel HTTP requests using predefined payloads to test optimistic locking

## Commands

### Build & Run

- Supported flags:
  - Persistent HTTP thread count: `-threads`
  - Enable debug output of HTTP request and response: `-debug`

```shell
mkdir -p ./bin
env GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./bin . && ./bin/lock-tester.exe --threads=200
```

### Build & Run from a Shell script

```shell
./run.sh
```
