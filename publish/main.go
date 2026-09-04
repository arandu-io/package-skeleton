// Command publish copies this package's views into an Arandu application.
//
// Run it from anywhere inside the project that installed the package:
//
//	go run github.com/arandu-io/package-skeleton/publish@latest
//	go run github.com/arandu-io/package-skeleton/publish@latest --force
//
// It writes the view sources under resources/views/vendor/, prints the import
// lines that put them in the binary, and stops there. It compiles nothing, runs
// nothing and edits no file the project already had.
//
// It is a command of this module rather than a subcommand of the CLI, and that
// is what makes the line above enough: the project adds no dependency to run
// it, and has nothing to remove afterwards.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	skeleton "github.com/arandu-io/package-skeleton"
)

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	switch {
	case err == nil:
		return
	case errors.Is(err, errReported):
		// The flag package already wrote the reason and the usage. A second
		// line here would say the same thing twice, under a different wording.
		os.Exit(1)
	default:
		fmt.Fprintln(os.Stderr, "publish:", err)
		os.Exit(1)
	}
}

// errReported is a failure the caller has already explained, so that the exit
// status is set without the message being written a second time.
var errReported = errors.New("reported")

// usage is what the command answers -h with, and what a wrong flag prints.
const usage = `usage: ` + skeleton.PublishCommand + ` [--force]

Copies this package's views into the project they are installed in.

  --force   overwrite views the project already has
`

// run does the whole job, with its output and its arguments handed in so a test
// can drive it the way a person does.
func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("publish", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }
	force := flags.Bool("force", false, "overwrite views the project already has")
	switch err := flags.Parse(args); {
	case errors.Is(err, flag.ErrHelp):
		// Asking for the usage is not a failure, and exiting non-zero over it
		// stops a script that only wanted to print it.
		return nil
	case err != nil:
		return errReported
	}
	// A word after the flags is somebody asking for something this command does
	// not do, and doing the default silently is how they find that out from the
	// result instead of from the message.
	if flags.NArg() > 0 {
		return fmt.Errorf("this command takes no arguments, and %q was given.\n\n%s", flags.Arg(0), usage)
	}

	root, err := projectRoot()
	if err != nil {
		return err
	}
	return publish(root, skeleton.Publishes(), skeleton.PublishedPaths(), *force, stdout)
}

// publish writes the archive into the project, or refuses and writes nothing.
//
// The refusal is decided over the whole set before a byte is written. A copy
// that stopped halfway would leave a project holding some of this version's
// views and some of the last one's, which is the state that renders and is
// wrong -- and the person running it a second time cannot tell which half they
// have.
func publish(root string, archive fs.FS, paths []string, force bool, out io.Writer) error {
	var present []string
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
			present = append(present, path)
		}
	}
	if len(present) > 0 && !force {
		return fmt.Errorf("this project already has %s.\n\nNothing was written. Rerun with --force to overwrite %s, which replaces the file and every edit in it",
			strings.Join(present, ", "), plural(len(present), "it", "them"))
	}

	for _, path := range paths {
		body, err := fs.ReadFile(archive, path)
		if err != nil {
			return fmt.Errorf("reading %s out of the package: %w", path, err)
		}
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "  wrote   %s\n", path)
	}
	fmt.Fprintf(out, "\n%d file(s) published.\n", len(paths))

	return report(root, out)
}

// report prints what is left for a person to do, and it is left to a person on
// purpose.
//
// Compiling the views is the CLI's job and importing them is the project's, and
// this command does neither: it declares exec = false, so it may not run the
// CLI, and a generator that edits bootstrap/app.go behind somebody's back is a
// generator whose output nobody can explain.
func report(root string, out io.Writer) error {
	module, err := readModulePath(root)
	if err != nil {
		return err
	}

	fmt.Fprint(out, "\nCompile them:\n\n    aru view:build\n")
	fmt.Fprint(out, "\nThen add to the imports of bootstrap/app.go, one line per directory:\n\n")
	for _, pkg := range skeleton.ViewPackages() {
		fmt.Fprintf(out, "    _ %q\n", module+"/"+pkg)
	}
	fmt.Fprint(out, "\nWithout the import the views are not in the binary, and the module refuses to boot and says so.\n")
	return nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// projectRoot walks up from the working directory to the project.
//
// A project is go.mod, main.go and arandu.toml together. Any one of them alone
// is something else -- a Go module, a program, or a directory somebody copied a
// configuration file into -- and writing views into one of those leaves files
// nothing will ever compile.
func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isProject(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("this is not an Arandu project: no go.mod, main.go and arandu.toml together.\n" +
				"Run it from inside a project, or create one with `aru new`")
		}
		dir = parent
	}
}

func isProject(dir string) bool {
	for _, name := range []string{"go.mod", "main.go", "arandu.toml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return false
		}
	}
	return true
}

// errModulePath is what a go.mod with no module line leaves behind, which is a
// file that is not a go.mod.
var errModulePath = errors.New("the project's go.mod names no module, so there is no import path to print")

// readModulePath reads the module path out of the project's go.mod.
//
// Line by line rather than parsed: one line of one file is wanted, and a parser
// for it would be the first dependency this module adds to everybody who runs
// the command.
func readModulePath(root string) (string, error) {
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(body), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", errModulePath
}
