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

grade name executable:
    go run ./cmd/grade {{quote(name)}} {{quote(executable)}}

cover:
    go test -coverprofile coverage.txt ./...
    go tool cover -func coverage.txt

# Validate the release configuration and build the release archives locally into dist/.
snapshot:
    goreleaser check
    goreleaser release --snapshot --clean

[unix]
clean:
    rm -rf bin dist coverage.txt

[windows]
clean:
    foreach ($p in 'bin', 'dist', 'coverage.txt') { if (Test-Path $p) { Remove-Item -Recurse -Force $p } }
