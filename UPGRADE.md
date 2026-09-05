# Upgrade Guide

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
