// Command touch changes file access and modification times, creating files
// that do not exist.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/GyeongHoKim/build-your-own-coreutils/internal/filetime"
	"github.com/GyeongHoKim/build-your-own-coreutils/internal/touch"
)

const usage = "usage: touch [-a] [-m] [-c] [-r reference | -t time] [--] file ...\n"

const help = usage + `
Update the access and modification times of each file to the current time.
A file that does not exist is created empty.

  -a            change only the access time
  -m            change only the modification time
  -c            do not create files that do not exist
  -r reference  use the times of reference instead of the current time
  -t time       use [[CC]YY]MMDDhhmm[.ss] in local time instead of the current time
  -h, --help    show this help

Options must come before the first file and cannot be combined (use -a -m,
not -am). -r and -t cannot be used together. "-" is an ordinary file name.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runAt(args, stdout, stderr, time.Now())
}

func runAt(args []string, stdout, stderr io.Writer, now time.Time) int {
	warn := func(format string, a ...any) { _, _ = fmt.Fprintf(stderr, "touch: "+format+"\n", a...) }
	usageError := func(format string, a ...any) int {
		warn(format, a...)
		_, _ = io.WriteString(stderr, usage)
		return 2
	}

	var (
		opts      touch.Options
		reference string
		stamp     string
	)
	flags := flag.NewFlagSet("touch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&opts.Access, "a", false, "")
	flags.BoolVar(&opts.Modify, "m", false, "")
	flags.BoolVar(&opts.NoCreate, "c", false, "")
	flags.StringVar(&reference, "r", "", "")
	flags.StringVar(&stamp, "t", "", "")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			if _, err := io.WriteString(stdout, help); err != nil {
				warn("write help: %v", err)
				return 1
			}
			return 0
		}
		return usageError("%v", err)
	}

	set := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch {
	case set["r"] && set["t"]:
		return usageError("cannot use both -r and -t")
	case set["r"] && reference == "":
		return usageError("empty reference file name")
	case set["t"] && stamp == "":
		return usageError("empty time")
	case flags.NArg() == 0:
		return usageError("missing file operand")
	}

	times := filetime.Times{Access: now, Modify: now}
	switch {
	case set["t"]:
		t, err := touch.ParseTime(stamp, now)
		if err != nil {
			return usageError("%v", err)
		}
		times = filetime.Times{Access: t, Modify: t}
	case set["r"]:
		var err error
		times, err = filetime.Read(reference)
		if err == nil {
			err = touch.Validate(times, opts)
		}
		if err != nil {
			warn("failed to get attributes of '%s': %v", reference, cause(err))
			return 1
		}
	}

	status := 0
	for _, path := range flags.Args() {
		if err := touch.Update(path, times, opts); err != nil {
			warn("cannot touch '%s': %v", path, cause(err))
			status = 1
		}
	}
	return status
}

func cause(err error) error {
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return pathErr.Err
	}
	return err
}
