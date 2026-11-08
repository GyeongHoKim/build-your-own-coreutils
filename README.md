<p align="center">
  <img src="assets/cover.png" alt="A Go gopher building things with a magnifying headlamp and a welding torch" width="240">
</p>

# build your own coreutils

<sub>Cover image: [egonelbre/gophers](https://github.com/egonelbre/gophers) (CC0).</sub>

> The most effective way to learn Go: build it yourself and figure it out along the way

This repository is the codebase for GyeongHo Kim's YouTube series "Build my own coreutils".

@TODO Add the playlist URL here

## Install on Windows

Release builds for Windows (amd64 and arm64) are published through [Scoop](https://scoop.sh/):

```powershell
scoop bucket add gyeongho https://github.com/GyeongHoKim/scoop-bucket
scoop install gyeongho/gcoreutils
```

The `gcoreutils` package installs every released utility with a `g` prefix, such as `gtouch` and `gtree`, so they do not shadow Windows built-ins like `tree.com`. Diagnostics still use the original utility name, for example `touch: cannot touch ...`. You can also download the zip archives from [GitHub Releases](https://github.com/GyeongHoKim/build-your-own-coreutils/releases).

## Learn with your AI tutor!

Implement your own utilities in `playground/<utility>/` and use the `coreutils-tutor` skill for guidance and feedback.

## Check your work

Build your program, then pass the utility name and your executable to the grader from the repository root:

```sh
# Windows
go build -o bin/my-<utility>.exe ./playground/<utility>
just grade <utility> ./bin/my-<utility>.exe

# Linux and macOS
go build -o bin/my-<utility> ./playground/<utility>
just grade <utility> ./bin/my-<utility>
```

The grader runs your program as a black box and never reads or changes your code. Each case prints one line:

- `PASS <case>`
- `FAIL <case>: <reason>`, with what the case expected and what your program did
- `SKIP <case>: <why this machine cannot run it>`

Skipped cases are left out of the totals, so a skip means less coverage, not a pass.

The grader exits with `0` when every executed case passes, `1` when any case fails, and `2` for invalid usage, a program that cannot be started, a fixture problem, or when no case could run. Passing covers only the published cases; it does not prove complete compatibility, or correct behavior on operating systems where you did not run it.

## Utilities

Each implemented utility has a page next to its code, `cmd/<utility>/README.md`, with usage examples and differences from the GNU version. Build the reference implementation with `just build <utility>`.

| Name | Core algorithm | Go standard library to study |
|---|---|---|
| tree | Recursive DFS, indentation prefix calculation | `os`, `io/fs`, `path/filepath`, `sort` |
| [touch](cmd/touch/README.md) | Timestamp selection, strict date parsing | `os`, `io/fs`, `time`, `flag`, `syscall` |
| cat | Streaming copy | `os`, `io`, `bufio` |
| wc | Single-pass counting, UTF-8 decoding | `bufio`, `unicode/utf8`, `unicode` |
| head / tail | Line buffering, ring buffer, reverse seek from the end of a file | `bufio`, `os`, `io` (`Seeker`) |
| ls | Directory listing, sorting, file mode formatting | `os`, `io/fs`, `sort`, `text/tabwriter` |
| cp / rm | Recursive walk, safe overwrite handling | `os`, `io`, `path/filepath` |
| grep | Line scanning, regex matching | `regexp`, `bufio`, `flag` |
| find | Tree traversal combined with predicates | `io/fs`, `path/filepath`, `time` |
| sort / uniq | External merge sort, adjacent duplicate removal | `sort`, `container/heap`, `bufio` |
| xargs | Command batching, process execution | `os/exec`, `bufio`, `context` |
| nc | TCP connections, bidirectional stream copying | `net`, `io`, `os`, goroutines |
| whois | TCP request and response streaming | `net`, `io`, `time` |
| curl | HTTP requests, response streaming | `net/http`, `io`, `net/url` |
