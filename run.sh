#!/bin/bash

set -e

if [[ "$1" == "-h" || "$1" == "--help" ]]; then
  echo "Usage: $0 [-threads NUMBER] [-debug]"
  echo "  -threads NUMBER  Set number of threads for the application (default: 50)"
  echo "  -debug           Enable debug mode"
  exit 0
fi

THREADS=""
DEBUG=""

while [[ $# -gt 0 ]]; do
  case $1 in
    -threads)
      THREADS="$2"
      shift 2
      ;;
    -debug)
      DEBUG="true"
      shift
      ;;
    *)
      echo "Unknown option $1"
      exit 1
      ;;
  esac
done

echo "Checking and installing Go quality tools if needed..."
if ! command -v staticcheck &> /dev/null; then
  echo "Installing staticcheck..."
  go install honnef.co/go/tools/cmd/staticcheck@latest
fi

if ! command -v ineffassign &> /dev/null; then
  echo "Installing ineffassign..."
  go install github.com/gordonklaus/ineffassign@latest
fi

if ! command -v golangci-lint &> /dev/null; then
  echo "Installing golangci-lint..."
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

echo "Running quality checks..."
go fmt ./...
golangci-lint run

echo "Building and running..."
mkdir -p ./bin
env GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./bin ./cmd/lock-tester 

ARGS=""
if [[ -n "$THREADS" ]]; then
  ARGS="$ARGS -threads $THREADS"
fi
if [[ "$DEBUG" == "true" ]]; then
  ARGS="$ARGS -debug"
fi

./bin/lock-tester.exe $ARGS