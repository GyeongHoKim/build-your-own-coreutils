<p align="center">
  <img src="assets/cover.png" alt="A Go gopher building things with a magnifying headlamp and a welding torch" width="240">
</p>

# build your own coreutils

<sub>Cover image: [egonelbre/gophers](https://github.com/egonelbre/gophers) (CC0).</sub>

> The most effective way to learn Go: build it yourself and figure it out along the way

This repository is the codebase for GyeongHo Kim's YouTube series "Build my own coreutils".

@TODO Add the playlist URL here

## Learn with your AI tutor!

Implement your own utilities in `playground/` and use the `coreutils-tutor` skill for guidance and feedback.

## Table

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
