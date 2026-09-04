package skeleton

import (
	"embed"
	"io/fs"
	"strings"
)

// The view sources this package hands to the application that installs it.
//
// They are embedded rather than read off disk because the command that copies
// them runs straight from the module proxy: `go run <module>/publish@latest`
// downloads the module, builds it and executes it, and at that point nothing of
// this repository exists as files. Whatever the command writes has to already
// be inside the binary it just built.
//
// The paths inside the archive are the paths the files take in the application,
// not paths of this repository that something has to translate on the way. A
// view sits here at the address it will sit at there, so publishing is a copy
// and there is no second spelling of a destination for the first one to
// disagree with.
//
//go:embed resources/views
var viewSources embed.FS

// Where a view is written and what it is called.
//
// viewRoot is the directory every path in the archive starts with, and it is
// what is cut off to get the name a view is registered under. viewSuffix is the
// extension: it ends in .go so the build tag on the first line keeps the
// compiler out of a file that is markup below the package clause.
const (
	viewRoot   = "resources/views"
	viewSuffix = ".kyse.go"
)

// compiledRoot is where the view compiler writes the Go it produces, mirroring
// the tree of the sources. It is build output: gitignored, rebuilt on demand,
// and never edited.
const compiledRoot = "storage/framework/views"

// vendorDir is the directory an application keeps other people's views in.
//
// It is part of the path in the archive and not something the publisher adds,
// which is what makes the archive a literal picture of what lands in the
// project. Two packages with a view called index are two files under two names
// below it, and neither shadows the other or the application's own.
const vendorDir = "vendor"

// Publishable is what a module implements to hand files to the application.
//
// It is an optional capability, discovered the way every other one is: the
// value is already held as a module, and whoever wants the files asks whether
// it also answers this. A module that publishes nothing does not implement it,
// and nothing changes for it.
//
// The signature names only io/fs, so the interface can be declared in more than
// one place and still be one interface to the compiler. Nothing here has to
// import the declaration it is measured against.
//
// Name is in the contract because the destination is derived from it: what the
// module calls itself is the directory its files land in, and a second name
// kept elsewhere for the same purpose is the pair that drifts.
type Publishable interface {
	Name() string
	Publishes() fs.FS
}

// Compile-time proof that the module honors the contract it claims.
var _ Publishable = (*Module)(nil)

// Publishes returns the files this package offers, each at the path it takes
// relative to the root of the application.
//
// Views and nothing else, and each absence is a decision rather than an
// omission:
//
//   - configuration is the Config struct New is handed, checked by the compiler
//     and validated before the module exists. A file copied into the project
//     beside it would be a second place to say the same thing, and only one of
//     the two could be the one the code reads.
//   - a stylesheet, a script or any other asset is registered with the view
//     layer and served from an address derived from its own bytes. Copying one
//     into the project would put a second copy of those bytes under a second
//     address, and a page can only reference one of them.
//   - translations are overridden by writing the lines the application wants
//     into its own vendor tree, which the catalogue loader already reads. A
//     copy of every line this package ships is a copy that goes stale, and it
//     goes stale without saying so.
//   - migrations are declared and collected, never copied. A copy in the
//     project's own migration directory is found by the runner as well, so one
//     schema change applies twice under two names.
//
// What is left is the markup, and it is here for the reason the others are not:
// it is the one thing the application is expected to edit. A package cannot
// know what a screen should say in a product it has never seen.
func Publishes() fs.FS { return viewSources }

// Publishes is the same set, reached through the module.
//
// Both forms exist because neither caller can reach the other's: the publishing
// command has no database handle and no session store, so it can never hold a
// Module, and the application holds nothing else. One implementation answers
// both, so there is no second archive to fall behind.
func (m *Module) Publishes() fs.FS { return Publishes() }

// PublishedPaths are the files in the archive, each relative to the root of the
// application, sorted.
func PublishedPaths() []string { return append([]string(nil), publishedPaths...) }

// ViewNames are the names the published views are rendered by, sorted.
//
// The name is the path under the view root with its separators turned into
// dots, which is what the view compiler writes into the registration call. It
// is derived from the same archive the publisher copies, so a view that was
// renamed cannot keep an old name here.
func ViewNames() []string { return append([]string(nil), viewNames...) }

// ViewPackages are the directories the compiled views land in, each relative to
// the root of the application, sorted and without repeats.
//
// Importing one is what puts its views in the binary: a compiled view calls the
// registry from init(), and a package nothing imports is not linked at all. The
// publisher prints these instead of writing them into bootstrap/app.go, because
// one line somebody reads beats a file that changed while they were not looking.
func ViewPackages() []string {
	var out []string
	seen := make(map[string]bool)
	for _, path := range publishedPaths {
		dir := compiledRoot + strings.TrimPrefix(path[:strings.LastIndexByte(path, '/')], viewRoot)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

// The archive, read once at load.
//
// A failure here is a broken binary rather than a condition to recover from:
// the files are compiled in, so either they are all there or the build that
// produced this program was wrong.
var publishedPaths, viewNames = readArchive()

func readArchive() (paths, names []string) {
	err := fs.WalkDir(viewSources, viewRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, viewSuffix) {
			return nil
		}
		paths = append(paths, path)
		names = append(names, viewName(path))
		return nil
	})
	if err != nil {
		panic("skeleton: reading the embedded views: " + err.Error())
	}
	if len(paths) == 0 {
		panic("skeleton: the embedded view directory holds no view, so every name this package renders would be missing and nothing would say so")
	}
	return paths, names
}

// viewName turns an archive path into the name the view is registered under.
//
//	resources/views/vendor/skeleton/index.kyse.go -> vendor.skeleton.index
func viewName(path string) string {
	name := strings.TrimPrefix(strings.TrimPrefix(path, viewRoot), "/")
	name = strings.TrimSuffix(name, viewSuffix)
	return strings.ReplaceAll(name, "/", ".")
}
