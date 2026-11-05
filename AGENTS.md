# Repository guidance

This repository accompanies a Go learning series that implements Unix-style command-line utilities. The long-term goal is to distribute useful Windows binaries through package managers such as Scoop and winget. Keep the code approachable for learners and suitable for real use.

## Go principles

- Favor clear, simple, idiomatic Go over cleverness, speculative abstractions, and framework-like designs. Make the zero value useful where practical.
- Use [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) as a primary reference when writing and reviewing Go code; consult the relevant sections when making style decisions. It supplements [Effective Go](https://go.dev/doc/effective_go), rather than serving as a complete style guide.
- Follow `gofmt`. Use descriptive package and exported names, short names where scope makes them clear, and conventional initialisms such as `URL` and `ID`.
- Prefer the standard library when it fits. Add dependencies only when they provide a clear benefit; explain significant additions.
- Keep packages focused and APIs small. Define interfaces where they are consumed when useful; do not introduce interfaces solely for abstraction or testing.
- Handle errors explicitly, adding context where useful and preserving the underlying error with `%w` when callers need to inspect it. Avoid panic for ordinary failures and avoid silently discarding meaningful errors.
- Document exported declarations with Go doc comments that explain their purpose or contract. Add other comments for intent and non-obvious tradeoffs, not a narration of the code.
- Prefer straightforward control flow and concrete types. Use concurrency only when it solves a demonstrated problem, and ensure goroutines have a clear lifetime.

## Commands and portability

- Keep command-line parsing, diagnostics, and exit status in `cmd/<name>`; place reusable command behavior in a focused internal package where appropriate.
- Treat stdout as command output and stderr as diagnostics. Handle I/O failures and test exit status and error cases where relevant.
- Follow the original Unix utility's documented behavior and specifications. Identify the reference implementation or standard when behavior differs, and document supported options and intentional differences; do not claim complete compatibility without evidence.
- Implement command logic in pure Go, without cgo or runtime dependencies on a Unix shell or external Unix utilities. Prefer portable standard-library APIs such as `path/filepath` for filesystem paths.
- Go cross-compilation does not remove operating-system differences. Consider and test platform-specific behavior when relevant, and keep release and packaging decisions explicit.

## Changes and verification

- Follow existing repository structure and conventions unless there is a concrete reason to change them. Keep changes scoped and explain consequential tradeoffs.
- Add or update focused, behavior-oriented tests for changed functionality, including relevant edge cases and platform-specific behavior. Prefer table-driven tests when they improve clarity; use temporary directories and injected I/O where appropriate.
- Use the `justfile` recipes: run `just fmt`, `just lint`, and `just test` for Go changes. Run `just build` or `just build <name>` and relevant platform checks when warranted; use `just cover` when coverage is useful. Report checks that could not be run.
- Update documentation when command behavior, supported options, build instructions, or distribution changes.
