# The cech command line on Cobra

How `cech` parses its command line, prints help and completion scripts, and reports a wrong command line, once it runs on Cobra.
The ADRs behind it are ADR 15 item 5 and ADR 16 items 1 and 3; what `cech build` does is in the spec of `cech build` under `done/`.

## Commands and flags

1. The commands are `build`, `completion`, `help` and `version`.
   Cobra's hidden `__complete` and `__completeNoDesc` serve the completion scripts in Cobra's own protocol; they are not commands for users, and rules 5 to 10 do not apply to them.
2. Long flags take two dashes, as `--check` or `--json=false`; a long flag with one dash, as `-check`, is a wrong command line.
3. `-C dir` names the directory to work in, with `--directory dir` as its long form.
4. `-h` and `--help`, before or after a command, print that command's help on stdout and exit 0, as `cech help <command>` does; `<command>` may be a path, as `completion bash`.
   `cech help` and `cech -h` print the help of `cech`.

## A wrong command line

5. Each of these exits 2, prints nothing on stdout, and prints on stderr a message that starts with `cech:`, names the problem and ends with `Run cech help for usage.`:
   - no command, as `cech`, `cech --` or `cech --help=false`;
   - an unknown command, or an unknown command after `help`;
   - an unknown flag, or a flag value of the wrong type;
   - an argument to `build` or `version`;
   - `completion` without a shell, or with a shell it does not know;
   - `build` with both `--check` and `--dry-run` set, with a `--release` that is not `X.Y.Z` or `X.Y.Z-rc.N`, or with an empty `-C`.
6. A wrong command line prints no JSON, even with `--json`: ADR 15 item 5 asks for the object only after the arguments parse, and every case of rule 5, including the checks of `build`'s flags in its last item, is a failure to parse them.

## Completion

7. `cech completion bash`, `zsh`, `fish` and `powershell` each print Cobra's completion script for that shell on stdout and exit 0.
8. `completion` and its shells take no `--json` (ADR 15 item 5); each shell takes Cobra's `--no-descriptions`.

## Output

9. A failed write of help or of a completion script, as to a closed pipe, exits 1 with `cech: cannot write the output: <error>` on stderr, as a failed write of any other result does.
10. Cobra prints no errors or usage of its own: every message on stderr is in the form of rule 5 or comes from the command.

## Tests

11. One golden file, `cli/internal/app/testdata/commandline.golden`, holds the help of every command but the hidden ones and the shells of `completion`, whose help is Cobra's, found by walking the command tree; a command added to the tree adds itself to the file.
12. Tests cover what cech's own code and choices do under rules 2 to 10, and `TestCommandLine` covers rules 1 and 11.
    They do not repeat what Cobra or pflag guarantee, such as which help `-h` and `--help` print after a command, how a flag value is parsed, or the script of each shell.
