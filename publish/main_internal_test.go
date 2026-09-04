package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	skeleton "github.com/arandu-io/package-skeleton"
)

// What publishing has to be, proved against a project on disk rather than
// described:
//
//  1. a view the project does not have is written, at the path the package
//     names for it;
//  2. a view the project already has is not touched, and the refusal says which
//     file and which flag;
//  3. a refusal writes nothing at all, not even the files that were free to go;
//  4. --force replaces the file, edits included, because that is what the
//     refusal said it would do;
//  5. nothing is written outside a project.
//
// The archive is the real one. A fixture would prove that this test can copy
// files, and the property worth holding is that what the package offers is what
// lands.

// project writes the three files that make a directory an Arandu project and
// returns it. The contents do not matter: what is read here is the module line,
// and what is checked is that all three are present.
func project(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":      "module example.test/app\n\ngo 1.26\n",
		"main.go":     "package main\n\nfunc main() {}\n",
		"arandu.toml": "[arandu]\naru = \"v0.35.0\"\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return root
}

// firstView is a path the package publishes, and the one the tests below edit.
func firstView(t *testing.T) string {
	t.Helper()

	paths := skeleton.PublishedPaths()
	if len(paths) == 0 {
		t.Fatal("the package publishes nothing, so every test below would pass by having no file to write")
	}
	return paths[0]
}

func TestPublishWritesEveryViewIntoTheProject(t *testing.T) {
	root := project(t)
	var out bytes.Buffer

	if err := publish(root, skeleton.Publishes(), skeleton.PublishedPaths(), false, &out); err != nil {
		t.Fatalf("publishing into an empty project: %v", err)
	}

	for _, path := range skeleton.PublishedPaths() {
		written, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("reading the published %s: %v", path, err)
			continue
		}
		if len(written) == 0 {
			t.Errorf("%s was published empty", path)
		}
		if !strings.Contains(out.String(), path) {
			t.Errorf("the report does not name %s", path)
		}
	}
}

// TestPublishPrintsTheImportThatLinksTheViews holds the half of the install a
// person has to do, because a view nobody imports is not in the binary and the
// module refuses to boot over it.
func TestPublishPrintsTheImportThatLinksTheViews(t *testing.T) {
	root := project(t)
	var out bytes.Buffer

	if err := publish(root, skeleton.Publishes(), skeleton.PublishedPaths(), false, &out); err != nil {
		t.Fatalf("publishing into an empty project: %v", err)
	}

	report := out.String()
	packages := skeleton.ViewPackages()
	if len(packages) == 0 {
		t.Fatal("the package compiles into no view package, so there is no import to print")
	}
	for _, pkg := range packages {
		want := `_ "example.test/app/` + pkg + `"`
		if !strings.Contains(report, want) {
			t.Errorf("the report does not print the import %s", want)
		}
	}
	if !strings.Contains(report, "aru view:build") {
		t.Error("the report does not name the command that compiles what was just written")
	}
}

func TestPublishRefusesToReplaceAViewTheProjectHas(t *testing.T) {
	root := project(t)
	owned := firstView(t)

	edited := []byte("//go:build kyse\n\npackage skeleton\n\n<p>mine</p>\n")
	full := filepath.Join(root, filepath.FromSlash(owned))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("preparing %s: %v", owned, err)
	}
	if err := os.WriteFile(full, edited, 0o644); err != nil {
		t.Fatalf("writing %s: %v", owned, err)
	}

	var out bytes.Buffer
	err := publish(root, skeleton.Publishes(), skeleton.PublishedPaths(), false, &out)
	if err == nil {
		t.Fatal("publishing over a view the project already has was allowed")
	}
	if !strings.Contains(err.Error(), owned) {
		t.Errorf("the refusal does not name the file: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("the refusal does not say how to override it: %v", err)
	}

	if after, readErr := os.ReadFile(full); readErr != nil || !bytes.Equal(after, edited) {
		t.Error("the refusal replaced the file it refused to replace")
	}
	if out.Len() > 0 {
		t.Errorf("the refusal reported writing something: %q", out.String())
	}
}

// TestARefusalWritesNothingAtAll holds the part of the refusal a project with
// one view cannot show: that the file which was free to go did not go either.
//
// Half of one version beside half of the next is the state that compiles,
// renders, and is wrong -- and the person who reruns the command cannot tell
// from the tree which half they are holding. The archive is made here rather
// than taken from the package, because the property belongs to the copy and has
// to hold at two files whatever the package happens to ship today.
func TestARefusalWritesNothingAtAll(t *testing.T) {
	root := project(t)
	const owned = "resources/views/vendor/example/one.kyse.go"
	const free = "resources/views/vendor/example/two.kyse.go"

	archive := fstest.MapFS{
		owned: {Data: []byte("packaged one\n")},
		free:  {Data: []byte("packaged two\n")},
	}

	full := filepath.Join(root, filepath.FromSlash(owned))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("preparing %s: %v", owned, err)
	}
	if err := os.WriteFile(full, []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", owned, err)
	}

	var out bytes.Buffer
	if err := publish(root, archive, []string{owned, free}, false, &out); err == nil {
		t.Fatal("publishing over a file the project already has was allowed")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(free))); err == nil {
		t.Error("the refusal still wrote the file it had no reason to refuse")
	}
}

func TestForceReplacesTheViewTheProjectHas(t *testing.T) {
	root := project(t)
	owned := firstView(t)
	full := filepath.Join(root, filepath.FromSlash(owned))

	edited := []byte("//go:build kyse\n\npackage skeleton\n\n<p>mine</p>\n")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("preparing %s: %v", owned, err)
	}
	if err := os.WriteFile(full, edited, 0o644); err != nil {
		t.Fatalf("writing %s: %v", owned, err)
	}

	var out bytes.Buffer
	if err := publish(root, skeleton.Publishes(), skeleton.PublishedPaths(), true, &out); err != nil {
		t.Fatalf("publishing with --force: %v", err)
	}

	after, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading %s: %v", owned, err)
	}
	if bytes.Equal(after, edited) {
		t.Error("--force left the edited file in place, so the flag the refusal names does nothing")
	}

	packaged, err := fs.ReadFile(skeleton.Publishes(), owned)
	if err != nil {
		t.Fatalf("reading %s out of the package: %v", owned, err)
	}
	if !bytes.Equal(after, packaged) {
		t.Error("--force wrote something other than what the package carries")
	}
}

func TestNothingIsWrittenOutsideAnAranduProject(t *testing.T) {
	t.Chdir(t.TempDir())

	var out bytes.Buffer
	err := run(nil, &out, &out)
	if err == nil {
		t.Fatal("a directory that is not a project was published into")
	}
	for _, want := range []string{"go.mod", "main.go", "arandu.toml"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say what a project is: %q is missing from %v", want, err)
		}
	}
}

func TestAnArgumentIsRefusedRatherThanIgnored(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"views"}, &out, &out)
	if err == nil {
		t.Fatal("a word after the flags was accepted")
	}
	if !strings.Contains(err.Error(), "views") {
		t.Errorf("the refusal does not repeat what was given: %v", err)
	}
}

// TestAskingForTheUsageIsNotAFailure keeps -h out of the exit status. A script
// that only wanted to print the usage should not stop over having done so.
func TestAskingForTheUsageIsNotAFailure(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"-h"}, &out, &out); err != nil {
		t.Fatalf("asking for the usage failed: %v", err)
	}
	if !strings.Contains(out.String(), "--force") {
		t.Errorf("the usage does not name the flag the refusal points at: %q", out.String())
	}
}
