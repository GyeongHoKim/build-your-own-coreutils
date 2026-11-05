<p align="center">
  <img src="assets/cover.png" alt="A Go gopher building things with a magnifying headlamp and a welding torch" width="240">
</p>

# build your own coreutils

<sub>Cover image: [egonelbre/gophers](https://github.com/egonelbre/gophers) (CC0).</sub>

> The most effective way to learn Go: build it yourself and figure it out along the way

This repository is the codebase for GyeongHo Kim's YouTube series "Build my own coreutils".

@TODO Add the playlist URL here

## Table

| Name | Core algorithm | Go standard library to study | YouTube video |
|---|---|---|---|
| tree | Recursive DFS, indentation prefix calculation | `os`, `io/fs`, `path/filepath`, `sort` | [Watch](https://youtu.be/5J3RQ-N9w8Y) |
| cat | Streaming copy | `os`, `io`, `bufio` | Coming soon |
| wc | Single-pass counting, UTF-8 decoding | `bufio`, `unicode/utf8`, `unicode` | Coming soon |
| head / tail | Line buffering, ring buffer, reverse seek from the end of a file | `bufio`, `os`, `io` (`Seeker`) | Coming soon |
| ls | Directory listing, sorting, file mode formatting | `os`, `io/fs`, `sort`, `text/tabwriter` | Coming soon |
| cp / rm | Recursive walk, safe overwrite handling | `os`, `io`, `path/filepath` | Coming soon |
| grep | Line scanning, regex matching | `regexp`, `bufio`, `flag` | Coming soon |
| find | Tree traversal combined with predicates | `io/fs`, `path/filepath`, `time` | Coming soon |
| sort / uniq | External merge sort, adjacent duplicate removal | `sort`, `container/heap`, `bufio` | Coming soon |
| xargs | Command batching, process execution | `os/exec`, `bufio`, `context` | Coming soon |
