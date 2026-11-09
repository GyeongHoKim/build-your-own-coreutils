# rm

Remove files and, with `-r`, directories and their contents. Follows POSIX.1-2024.

```text
rm [-dfiRrv] [--] file ...
```

```sh
rm notes.txt                # remove a file
rm -f maybe.txt             # no error if it does not exist, never prompt
rm -d empty-dir             # remove an empty directory
rm -rv build                # remove build/ and everything in it, printing each removal
rm -ri old                  # ask before every removal
rm -- -draft.txt            # a file name that starts with -
```

Run `rm --help` for every option. Build it with `just build rm`.

## Behavior

- Options: `-d`, `-f`, `-i`, `-R`/`-r`, `-v`, and `-h`/`--help`. Short options may be grouped (`-rf`); the last of `-f` and `-i` wins.
- Options stop at the first operand. `--` ends them explicitly, and `-` is an ordinary file name.
- `-v` prints `removed 'x'` or `removed directory 'x'` for each removal.
- Prompts go to standard error and an answer starting with `y` or `Y` is affirmative. `rm` prompts when `-i` is given, or when a file is write-protected, `-f` is absent, and standard input is a terminal.
- Exit status is `0` on success, `1` if any removal failed, and `2` for a usage error.

## Safety

- `rm` refuses `.`, `..` as the last component, and the filesystem root, including drive roots such as `C:\`, UNC share roots, volume roots reached through a mount point or a `\\?\Volume{...}\` path, and a symbolic link or junction to the root written with a trailing slash. The root is recognized by identity, not spelling, and an operand whose root status cannot be determined is not removed.
- Symbolic links and junctions (and other name-surrogate reparse points, such as volume mount points) met during traversal are never followed; the link itself is removed.
- A directory replaced by a link while it is being removed is refused with "directory changed during removal".

## Known limits

- One directory handle stays open per level of depth being processed, so the depth of a hierarchy `rm` can remove is bounded by the open-file limit. POSIX requires `rm` to remove hierarchies of arbitrary depth.

## Differences from GNU rm

- Long options, `-I`, `--interactive`, `--one-file-system`, and the `--preserve-root` variants are not supported, and `-h` shows help.
- A declined entry makes its parent directory fail with exit status `1`, as POSIX literally specifies. GNU rm skips the parent silently.
- `rm -r link/`, where `link` is a symbolic link or junction to a directory, removes the target's contents and then fails with "Not a directory", keeping both the link and the target directory. `rm -d link/` fails with "Not a directory".
- An empty write-protected directory is removed without a prompt unless `-i` is given. POSIX allows this; GNU rm prompts.
- Siblings are removed in parallel, so the order of `-v` lines and diagnostics can vary between runs. `-i` is always sequential.

## Windows notes

- Files held open by Explorer or other programs are removed immediately, using POSIX delete semantics on NTFS (Windows 10 version 1607 or later). On file systems without them, such as FAT32, `rm` falls back to a normal delete.
- A file truly locked by a running program fails, and `rm` does not retry.
- The read-only attribute counts as write-protected for files only.
- In PowerShell, `rm` is an alias of `Remove-Item`. Use `grm` from the Scoop package.
