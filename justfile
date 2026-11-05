set windows-shell := ["pwsh.exe", "-NoLogo", "-NoProfile", "-Command"]
set shell := ["zsh", "-cu"]

default: list

list:
    @just --list

fmt:
    golangci-lint fmt ./...

lint:
    golangci-lint run ./...

build name="...":
    go build -o bin/ ./cmd/{{name}}

test:
    go test ./...

cover:
    go test -coverprofile coverage.txt ./...
    go tool cover -func coverage.txt

[unix]
clean:
    rm -rf bin coverage.txt

[windows]
clean:
    foreach ($p in 'bin', 'coverage.txt') { if (Test-Path $p) { Remove-Item -Recurse -Force $p } }
