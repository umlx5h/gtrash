# Configration

Certain behaviors can be altered by setting environment variables.  

## GTRASH_HOME_TRASH_DIR

- Type: string
- Default: `$XDG_DATA_HOME/Trash ($HOME/.local/share/Trash)`

Change the location of the main file system's trash can by specifying the full path.

Example: If you prefer placing it directly under your home directory:

```bash
export GTRASH_HOME_TRASH_DIR="$HOME/.gtrash"
```

## GTRASH_ONLY_HOME_TRASH

- Type: bool ('true' or 'false')
- Default: `false`

Enabling this option ensures the sole usage of the home directory's trash can.

When files from external file systems are deleted using the `put` command, they're copied to the trash can in `$HOME`. This process might take longer due to copying and increase the main file system's disk space.

By default (false), it searches for trash cans across all mount points and displays them using `find` and `restore` commands. This includes network and USB drives, potentially causing slower operation.

If you encounter such issues, enabling this option can be helpful.

```bash
export GTRASH_ONLY_HOME_TRASH="true"
```

## GTRASH_HOME_TRASH_FALLBACK_COPY

- Type: bool ('true' or 'false')
- Default: `false`

Enable this option to fallback to using the home directory's trash can when the external file system's trash can is unavailable. Enabling this option might resolve errors encountered while deleting files on an external file system using the `put` command.

It can also be set using the `--home-fallback` option.

```bash
$ gtrash put --home-fallback /external/file1

# Equivalent to the above
$ GTRASH_HOME_TRASH_FALLBACK_COPY="true" gtrash put /external/file1

# To disable it when enabled in the environment variable
$ GTRASH_HOME_TRASH_FALLBACK_COPY="true" gtrash put --home-fallback=false /external/file1
```

```bash
export GTRASH_HOME_TRASH_FALLBACK_COPY="true"
```

## GTRASH_PUT_RM_MODE

- Type: bool ('true' or 'false')
- Default: `false`

Enabling this option changes the behavior of the `put` command as closely as possible to `rm`.

The `-r`, `--recursive`, `-R`, `-d` options closely resemble `rm` behavior. When set to false, these options are completely ignored.

This setting can also be configured using the `--rm-mode` option.

```bash
$ gtrash put --rm-mode -r dir1/

# Equivalent to the above
$ GTRASH_PUT_RM_MODE="true" gtrash put -r dir/
```

```bash
export GTRASH_PUT_RM_MODE="true"
```
