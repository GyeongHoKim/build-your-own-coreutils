// Command tree lists the contents of directories in a tree-like format.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/tree"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) (status int) {
	var (
		opts     tree.Options
		noReport bool
	)

	warn := func(format string, a ...any) { _, _ = fmt.Fprintf(stderr, format, a...) }

	fs := flag.NewFlagSet("tree", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.All, "a", false, "list all files, including hidden ones")
	fs.BoolVar(&opts.DirsOnly, "d", false, "list directories only")
	fs.IntVar(&opts.MaxDepth, "L", 0, "descend only `level` directories deep")
	fs.BoolVar(&opts.DirsFirst, "dirsfirst", false, "list directories before files")
	fs.BoolVar(&noReport, "noreport", false, "omit the file and directory report at the end")
	fs.Usage = func() {
		warn("usage: tree [-a] [-d] [-L level] [--dirsfirst] [--noreport] [directory ...]\n")
		warn("note: short flags cannot be combined (use -a -d, not -ad)\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	fs.Visit(func(f *flag.Flag) {
		if f.Name == "L" && opts.MaxDepth < 1 {
			warn("tree: Invalid level, must be greater than 0.\n")
			status = 1
		}
	})
	if status != 0 {
		return status
	}

	dirs := fs.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}

	out := bufio.NewWriter(stdout)
	defer func() {
		if err := out.Flush(); err != nil {
			warn("tree: %v\n", err)
			status = 1
		}
	}()

	var total tree.Counts
	for _, dir := range dirs {
		counts, err := tree.Print(out, dir, opts)
		total = total.Add(counts)
		if err != nil {
			warn("tree: %v\n", err)
			status = 1
		}
	}

	if !noReport {
		if err := tree.Report(out, total, opts.DirsOnly); err != nil {
			warn("tree: %v\n", err)
			status = 1
		}
	}
	return status
}
