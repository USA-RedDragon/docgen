#!/bin/sh
set -eu
# docgen:region setup
go mod download
go build ./...
# docgen:end
go test ./...
