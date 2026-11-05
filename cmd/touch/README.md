# touch

Update the access and modification times of files, creating any that do not exist.

```text
touch [-a] [-m] [-c] [-r reference | -t [[CC]YY]MMDDhhmm[.ss]] [--] file ...
```

```sh
touch notes.txt                       # create notes.txt, or set its times to now
touch -c notes.txt                    # update only if it already exists
touch -m -r original.txt copy.txt     # copy only the modification time
touch -t 202611061430 report.txt      # 2026-11-06 14:30 in local time
touch -- -draft.txt                   # a file name that starts with -
```

Run `touch --help` for every option. Build it with `just build touch`.

## Differences from GNU touch

- `-d`/`--date`, `--time`, and `--no-dereference` are not supported, and `-h` shows help.
- Options cannot be combined (`-a -m`, not `-am`) and must come before the first file name.
- `-` is an ordinary file name, not standard output.
- Usage errors exit with status `2`; other failures exit with `1`.

## Windows notes

- Windows may not update access times on its own (`fsutil behavior query disablelastaccess`). Times set by `touch` are still stored.
- Creating symbolic links requires Developer Mode or administrator rights; `touch` follows existing links.

## Implement it yourself

Write your version in `playground/touch/`, then grade it as described in [Check your work](../../README.md#check-your-work):

```sh
just grade touch ./bin/my-touch.exe
```

Cases come in two tiers. Make every `core` case pass before working on `extended`. Each `FAIL` line says what the case expects, the arguments it used, and what your program did.
