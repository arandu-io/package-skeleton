# Upgrade Guide

## Unreleased

### Published views move out of `vendor/`

The view a package publishes lands in `resources/views/modules/<slug>/` and
compiles to `storage/framework/views/modules/<slug>`. It used to be `vendor/` in
both, and that address could not work: the go command reserves the name twice,
and a tree of published views hit both rules.

A file under a directory named `vendor` is left out of the module zip at any
depth. The file stays in the package's repository and is missing for everyone
who downloads it, so the `go:embed` that names its directory matches nothing and
the person building the project reads

```
pattern resources/views: no matching files found
```

— an error about the package, raised in their project. And a package whose
import path carries the element cannot be imported at all:

```
bootstrap/app.go:98:2: use of vendored package not allowed
```

which is exactly the import `(*Module).Boot` asks for. A published view is
compiled into a Go package the application has to import for its `init()` to
register anything, so the second rule refused the last step of the install.

Both were reproduced before this changed: `zip.CheckDir` reports the view as
`file is in vendor directory`, and a package under
`storage/framework/views/vendor/<slug>` is refused at import.

To move a package already released:

1. `git mv resources/views/vendor resources/views/modules`.
2. Rename the `vendorDir` constant in `views.go` to `moduleDir`, with the value
   `modules`.
3. Release the package, and tell the projects that installed it to publish
   again. The old files are theirs now, so `aru vendor:publish --apply` writes
   the new tree beside the old one and the old one is deleted by hand, along
   with its lines in `vendor-publish.lock` and its import in `bootstrap/app.go`.

Framework `v0.46.4` and Hesape `v0.37.0` refuse a publication that carries the
reserved name, so a package that has not moved fails its own tests with a
message naming both rules, rather than failing in the first project that
installs it.

<!-- configure:template-start -->
The notes below are the release history of the package skeleton this file was
cloned with. `configure` removes them.

## v0.5.0

Nothing to change in a package already configured from this repository. What
changed is what the next clone starts with: `configure` now removes this
repository's release history from `CHANGELOG.md` and `UPGRADE.md`, which it
previously renamed into the clone and left there.

A package that already carries it corrects its own two files and releases the
correction; `arandu-wallet` did it in `v0.4.1` and `arandu-tags` in `v0.2.3`.

## v0.4.0

Version 0.4.0 hands publishing to the framework. The package no longer defines
the contract or carries the command that writes the files. Upgrade Framework to
`v0.46.0` and Hesape to `v0.25.0` before changing anything below.

### Publish with the CLI

```sh
# Before.
go run :module_path/publish@latest
go run :module_path/publish@latest --force

# After.
aru vendor:publish --tag=view
aru vendor:publish --tag=view --apply
aru vendor:publish --tag=view --apply --force
```

The `publish` command of this module was removed. `aru vendor:publish` asks the
application which modules it registered and writes what each of them declares,
so one command publishes every installed package instead of one command per
package. Without `--apply` it writes nothing and prints what each file would
become; running it twice changes nothing the second time.

`PublishCommand` changed from `go run <module>/publish@latest` to
`aru vendor:publish --apply`. It is what `(*Module).Boot` names in its refusal,
and an application that prints it anywhere of its own gets the new spelling by
recompiling.

### Answer the framework's publishing contract

`Publishable`, declared by this package, was removed. The contract is
`foundation.Publishable` from `github.com/arandu-io/framework/foundation`, and
what it asks for is a list rather than a tree:

```go
// Before.
type Publishable interface {
	Name() string
	Publishes() fs.FS
}

// After.
type Publishable interface {
	Publishes() []foundation.Publication
}
```

`Module.Publishes` changed from `func() io/fs.FS` to
`func() []foundation.Publication`. A `Publication` carries the tag — one of
`view`, `component`, `config`, `migration`, `translation`, `asset` — the tree,
and optionally the directory to read it from and the directory it lands in. This
package declares one, tagged `foundation.PublishView`, with neither directory
set, because every path in its archive is already the path the file takes in the
project.

The package-level `Publishes` function was removed with the command that needed
it: it existed because a `package main` with no database handle could never hold
a `Module`, and there is no such command any more. Reach the declaration through
the module.

### Contracts that did not move

`PublishedPaths`, `ViewNames` and `ViewPackages` are unchanged, and so are the
paths the views land under. A project that already published them is holding the
same files at the same addresses; `aru vendor:publish` reports them as
unchanged rather than rewriting them.

## v0.3.1

Nothing to change. The notes for `v0.3.0` moved out of `Unreleased` and under
the heading that names them, which is where the release gate reads them from.

## v0.3.0

### Publish the views the package draws

`Publishable` and `Publishes()` arrive on `Module`, with `PublishedPaths`,
`ViewNames`, `ViewPackages` and `PublishCommand` derived from the archive rather
than written down separately.

```sh
go run <module>/publish@latest
```

`(*Module).Boot` refuses to serve when a view this package renders was never
published. It names the view and the command, rather than answering the first
request that reaches it with a 500 -- a missing view is a deployment that is not
finished, and the place to find that out is the boot.

## v0.2.0

Version 0.2.0 replaces the generic CRUD Repository with the configured
Model-first data path. Upgrade Framework to `v0.41.0` and Hesape to `v0.19.1`
before changing the package wiring.

### Replace Repository wiring

Construct the Service with the application database handle:

```go
// Before.
repository := NewSkeletonRepository(db)
service := NewSkeletonService(repository)

// After.
service := NewSkeletonService(db)
```

`SkeletonRepository` and `NewSkeletonRepository` were removed. The removed
generic CRUD methods are `(*SkeletonRepository).Create`,
`(*SkeletonRepository).Delete`, `(*SkeletonRepository).Find`,
`(*SkeletonRepository).List`, and `(*SkeletonRepository).Update`. Use
`Skeletons(db)` after authorization for generic CRUD. Add a Repository only for
a specialized query, report, projection, read model, export, or external
storage boundary.

### Keep Model results as pointers

The Service now returns the entities owned by the configured Model:

- `(*SkeletonService).Create` changed from `(Skeleton, error)` to
  `(*Skeleton, error)`;
- `(*SkeletonService).Find` changed from `(Skeleton, error)` to
  `(*Skeleton, error)`;
- `(*SkeletonService).List` changed from `([]Skeleton, error)` to
  `([]*Skeleton, error)`;
- `NewSkeletonService` changed from accepting `*SkeletonRepository` to
  accepting `*data.DB`.

Keep those pointers intact until converting them to `Resource` or `Collection`.
Copying an entity with an embedded Model can leave its internal entity pointer
attached to the original allocation.

`Skeleton`: old is comparable; new is not because it embeds
`model.Model[Skeleton]`. Do not use the entity as a map key or compare it with
`==`; compare stable fields such as `ID` instead.

### Contracts that did not move

`ErrNotFound`, route names, migration identity, `DefaultPrefix`, and
`DefaultPageSize` remain unchanged. Existing URLs and applied migrations do not
need translation.

## v0.1.0

The first release. Nothing to upgrade from.
<!-- configure:template-end -->
