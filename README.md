# Lock tester

## Purpose

- Command line utility that performs parallel HTTP requests using predefined payloads to test optimistic locking

## Commands

### Build & Run

- Supported arguments:
  - `-threads NUMBER` - Set number of threads for the application (default: 50)
  - `-debug` - Enable debug mode

```shell
mkdir -p ./bin
env GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./bin .

# With defaults
./bin/lock-tester.exe

# With threads
./bin/lock-tester.exe -threads 100

# With debug
./bin/lock-tester.exe -debug

# With threads and debug
./bin/lock-tester.exe -threads 200 -debug
```

### Build & Run from a Shell script

- Supported arguments:
  - `-threads NUMBER` - Set number of threads for the application (default: 50)
  - `-debug` - Enable debug mode

```shell
# With defaults
./run.sh

# With threads
./run.sh -threads 100

# With debug
./run.sh -debug

# With threads and debug
./run.sh -threads 200 -debug
```

> Can be combined with `tee` and outputted into a log file, e.g. `./run.sh -threads 50 2>&1 | tee "logs/run-$(date +%Y-%m_%d_%H%M%S)-50.log"`
