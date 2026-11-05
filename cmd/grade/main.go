// Command grade checks learner implementations against command behavior.
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
	"tree": gradeTree,
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
