// Command grade checks learner implementations against command behavior.
//
// Usage:
//
//	grade <utility> <executable>
//
// The executable is run as a black box in fresh temporary directories; only
// its exit status, stdout, stderr, and resulting files are inspected. Each
// case prints PASS, FAIL with what it expects and what happened, or SKIP
// with the reason the machine cannot run it.
//
// Cases grade only behavior on which POSIX.1-2024 and GNU agree, so a
// learner who follows either reference more closely is never penalized.
// Any non-zero status counts as an error, and diagnostics may use any
// program name as long as they mention the path involved. For touch this
// leaves out help, combined short options, options after operands, the "-"
// operand, seconds 60, and the 68/69 century boundary; explicit dates stay
// within 1970 through 2037.
package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const caseTimeout = 10 * time.Second

var graders = map[string]func(executable string, stdout, stderr io.Writer) int{
	"touch": gradeTouch,
	"tree":  gradeTree,
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || graders[args[0]] == nil {
		fmt.Fprintf(stderr, "usage: grade <%s> <executable>\n", strings.Join(slices.Sorted(maps.Keys(graders)), "|"))
		return 2
	}
	executable, err := filepath.Abs(args[1])
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	// The executable is explicitly selected by the user.
	if info, err := os.Stat(executable); err != nil || info.IsDir() { // #nosec G703 -- the user selects the executable to grade.
		fmt.Fprintf(stderr, "invalid executable: %s\n", executable)
		return 2
	}
	return graders[args[0]](executable, stdout, stderr)
}
