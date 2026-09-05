# Changelog

Everything worth knowing about a release of :package_name is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

## [0.4.0] - 2026-09-05

### Added

- `(*Module).Publishes` declares one `foundation.Publication`, tagged as a view.
  The contract belongs to the framework, so whatever writes the files reads
  every module through one interface instead of one this package defined for
  itself.

### Changed

- The minimum Framework version is now `v0.46.0`, with Hesape `v0.25.0`.
- `(*Module).Publishes` returns `[]foundation.Publication` instead of `io/fs.FS`.
- `PublishCommand` is now `aru vendor:publish --apply`.
- `(*Module).Boot` names the package whose import links the views, alongside the
  view and the command.

### Removed

- `Publishable`, the contract this package declared for itself.
  `foundation.Publishable` is the one it answers now.
- `Publishes`, the package-level function. There was a second form because a
  command with no database handle could not hold a `Module`; there is no such
  command any more.
- `publish`, the command of this module. `aru vendor:publish` reads the modules
  an application registered and writes what each one declares, which is a
  question only the application can answer.

## [0.3.1] - 2026-09-03

### Added

- `Publishable`, the optional contract a module answers to hand files to the
  application, and `Publishes()` on `Module`.
- `PublishedPaths`, `ViewNames` and `ViewPackages`, the three spellings of one
  view derived from the archive rather than written down separately.
- `PublishCommand`, the one spelling of the command that copies the views.
- `publish`, a command of this module: `go run <module>/publish@latest` writes
  the views under `resources/views/vendor/<module>/`, refuses to replace a file
  the project already has without `--force`, and prints the imports that link
  them.
- `(*Module).Boot` refuses to serve when a view this package renders was never
  published, naming the view and the command instead of answering the first
  request that reaches it with a 500.

## [0.2.0] - 2026-08-29

### Added

- `Skeletons(db)` exposes the configured, tenant-scoped Model used by the
  Service after authorization.

### Changed

- The minimum Framework version is now `v0.41.0`, with Hesape `v0.19.1`.
- `NewSkeletonService` now accepts `*data.DB` instead of
  `*SkeletonRepository`.
- `(*SkeletonService).Create` now returns `(*Skeleton, error)`.
- `(*SkeletonService).Find` now returns `(*Skeleton, error)`.
- `(*SkeletonService).List` now returns `([]*Skeleton, error)`.
- `Skeleton`: old is comparable; new is not because it embeds
  `model.Model[Skeleton]`. Compare stable fields such as `ID` instead.

### Removed

- `SkeletonRepository` and `NewSkeletonRepository`.
- `(*SkeletonRepository).Create`, `(*SkeletonRepository).Delete`,
  `(*SkeletonRepository).Find`, `(*SkeletonRepository).List`, and
  `(*SkeletonRepository).Update`. Add a Repository only for specialized
  queries, reports, projections, read models, exports, or external storage.
