// Command rm removes files and, with -R, directory trees.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/rm"
)

const usage = "usage: rm [-dfiRrv] [--] file ...\n"

const helpText = usage + `
Remove each file. Directories are removed only with -d or -R.

  -d            remove empty directories
  -f            never prompt and ignore files that do not exist
  -i            prompt before every removal
  -R, -r        remove directories and their contents recursively
  -v            print each removal
  -h, --help    show this help

Options may be grouped (-rf) and must come before the first file. The last
of -f and -i wins. "--" ends the options and "-" is an ordinary file name.
`

// stdinIsTerminal reports whether standard input is a terminal. Tests replace
// it. It uses x/term because checking os.ModeCharDevice alone also matches
// /dev/null and NUL.
var stdinIsTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	warn := func(format string, a ...any) { _, _ = fmt.Fprintf(stderr, "rm: "+format+"\n", a...) }
	usageError := func(format string, a ...any) int {
		warn(format, a...)
		_, _ = io.WriteString(stderr, usage)
		return 2
	}

	opts, operands, wantHelp, err := parseArgs(args)
	if err != nil {
		return usageError("%v", err)
	}
	if wantHelp {
		if _, err := io.WriteString(stdout, helpText); err != nil {
			warn("write help: %v", err)
			return 1
		}
		return 0
	}
	if len(operands) == 0 {
		if opts.Force {
			return 0
		}
		return usageError("missing operand")
	}

	opts.Terminal = stdinIsTerminal()
	answers := bufio.NewReader(stdin)
	writeFailed := false
	// promptFailed records that a question could not be asked or answered.
	// Every later question is then declined without being asked.
	promptFailed := false
	failPrompt := func(format string, err error) bool {
		promptFailed = true
		warn(format, err)
		return false
	}
	cb := rm.Callbacks{
		Confirm: func(question string) bool {
			if promptFailed {
				return false
			}
			if _, err := fmt.Fprintf(stderr, "rm: %s ", question); err != nil {
				// An answer to a question nobody saw is not consent.
				return failPrompt("write prompt: %v", err)
			}
			line, err := answers.ReadString('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return failPrompt("read standard input: %v", err)
			}
			line = strings.TrimSpace(line)
			return line != "" && (line[0] == 'y' || line[0] == 'Y')
		},
		Failed: func(path string, err error) {
			warn("cannot remove '%s': %v", path, err)
		},
	}
	if opts.Verbose {
		cb.Removed = func(path string, dir bool) {
			format := "removed '%s'\n"
			if dir {
				format = "removed directory '%s'\n"
			}
			if _, err := fmt.Fprintf(stdout, format, path); err != nil && !writeFailed {
				writeFailed = true
				warn("write standard output: %v", err)
			}
		}
	}
	if ok := rm.Remove(operands, opts, cb); !ok || writeFailed || promptFailed {
		return 1
	}
	return 0
}

// parseArgs parses rm options in the getopt style: options may be grouped,
// stop at the first operand, and "--" ends them. The last of -f and -i wins.
func parseArgs(args []string) (opts rm.Options, operands []string, help bool, err error) {
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			i++
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		if arg == "--help" {
			return opts, nil, true, nil
		}
		if arg[1] == '-' {
			return opts, nil, false, fmt.Errorf("unknown option %s", arg)
		}
		for _, c := range arg[1:] {
			switch c {
			case 'd':
				opts.Dirs = true
			case 'f':
				opts.Force, opts.Interactive = true, false
			case 'i':
				opts.Interactive, opts.Force = true, false
			case 'R', 'r':
				opts.Recursive = true
			case 'v':
				opts.Verbose = true
			case 'h':
				return opts, nil, true, nil
			default:
				return opts, nil, false, fmt.Errorf("unknown option -%c", c)
			}
		}
	}
	return opts, args[i:], false, nil
}
