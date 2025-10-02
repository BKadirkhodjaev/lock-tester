#!/bin/bash

set -e

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
go vet ./...
staticcheck ./...
ineffassign ./...
golangci-lint run

echo "Building and running..."
mkdir -p ./bin
env GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./bin . 
./bin/lock-tester.exe --threads=200