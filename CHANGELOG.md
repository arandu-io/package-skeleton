# Changelog

Everything worth knowing about a release of :package_name is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

<!-- configure:template-start -->
The entries below are the release history of the package skeleton this file
was cloned with. `configure` removes them, so a configured package starts with
an empty changelog rather than with somebody else's.

## [0.7.2] - 2026-10-09

### Changed

- Require Framework `v0.55.1` and Hesape `v0.52.0`, the latest published
  releases; `arandu.mod.toml` declares `framework = ">= 0.55"`. The package
  used none of the names Hesape `v0.52.0` removed nor the session path
  Framework `v0.55` retired, so no code changed.

## [0.7.1] - 2026-10-09

### Fixed

- The release workflow vetted `configure.go` outside the template section, so
  a configured package, which no longer has the file, failed the exact-archive
  check of the first tag it pushed. The line is inside a section of its own now
  and leaves with the file.
- `SECURITY.md` built the advisory address as
  `github.com/:author_username/:module_slug`, which is not the repository of a
  module published as `arandu-<slug>`. It is built from the module path.
- The upgrade notes for the `vendor/` move and the concrete model sat under an
  `Unreleased` heading outside the template section, and every configured
  package inherited them. They are under `v0.6.0` and `v0.7.0` inside it, and a
  configured package starts with an empty upgrade guide.
- `v0.6.0` had no changelog entry. It has one.

### Added

- `TestAConfiguredPackageCarriesNothingOfTheTemplatesOwn`, which runs
  `configure` on a copy of the tree as `github.com/hyz-is/arandu-widget` and
  reads the release workflow, `SECURITY.md`, `arandu.mod.toml` (whose name
  must be `hyz-is/widget`), `UPGRADE.md` and `CHANGELOG.md` that come out.

## [0.7.0] - 2026-10-09

### Changed

- **Breaking.** `Skeleton` embeds Hesape's non-generic `model.Model`. Its table
  is declared once, as `skeletonTable` in `model.go`, with `ManualKey` because
  the text primary key is written by the application, and `aru model:build`
  generates `SkeletonQuery.go` beside it: `Skeletons(model.DB) *SkeletonQuery`,
  `SkeletonQuery` and `SkeletonCollection`. The file is committed, and
  `aru model:build --check` says when it is stale. The fields and methods the
  generic model promoted onto `Skeleton` are gone; `UPGRADE.md` names all 154
  that the API diff reports against v0.6.2.
- **Breaking.** Every route is mounted behind `middleware.RequireAuth`. A
  visitor with no session is sent to `/auth/login` before any handler runs, and
  a handler reads who is asking with `ctx.User()`.
- Handlers return what the service returned and the framework's router answers
  it: a rejected input goes back with the messages in the flash (to a request
  that asks for JSON, 422 `application/problem+json`), a policy refusal is 403,
  and `ErrNotFound`, which now wraps `model.ErrModelNotFound`, is 404.
- The rules of the entity itself live in the custom block of `model.go`, with
  a `CanRename`/`Rename` pair as the example; the service validates, asks the
  policy, calls those rules and saves with the Grant.
- `configure` ends by telling the configured package to run `aru model:build`
  once, because the generator lays out a renamed query file by the length of
  the new name.
- The `skeleton-package` skill carries `audience: app` under `metadata`, which
  is what `aru skills:sync` reads to copy it into an application. `configure`
  keeps the mark, so every package cloned from here ships its `<slug>-package`
  skill marked and its other skills unmarked.
- Require Hesape `v0.50.1` and Framework `v0.51.0`; `arandu.mod.toml` declares
  `framework = ">= 0.51"`.

### Removed

- **Breaking.** `Config.Tenant`. It was the tenant of a guest, and with every
  route behind `RequireAuth` nothing reads it; the zero `Config` is valid.

### Added

- `TestThePackageUsesTheGeneratedConcreteModel`,
  `TestHandlersLeaveTheSubjectAndTheStatusToTheFramework` and
  `TestTheGuidesTeachTheGeneratedModelBoundary`, and feature tests that mount
  the module on a router with a flash: no session is a 303 to `/auth/login`, a
  signed-in administrator meets the closed policy's 403, an empty name goes
  back with the message in the flash, and a missing record is 404.

### Fixed

- `AGENTS.md` describes the generated query, where the entity's rules live,
  and the measured counts: 7 Go files and 43 tests.

## [0.6.2] - 2026-09-17

### Fixed

- Record the release entry before tagging so the exact-archive publication gate can publish it. The immutable v0.6.1 tag was left unchanged after its missing-entry refusal.
- Pin Framework v0.47.1 and Hesape v0.41.1, including the native UI initialization correction.
- Run the existing checks on develop before promotion to main.

## [0.6.0] - 2026-09-09

This entry was written after the tag, which shipped without one. The tag is
immutable and was left as it is.

### Changed

- **Breaking.** Published views live under `resources/views/modules/<slug>/`
  and compile to `storage/framework/views/modules/<slug>`. Under `vendor/` the
  go command left them out of the module zip and refused the import of the
  compiled package; the `vendorDir` constant in `views.go` is `moduleDir`.
- Require Framework `v0.46.4` and Hesape `v0.37.0`, which refuse a publication
  that carries the reserved name.

## [0.5.0] - 2026-09-06

### Changed

- The release history of this repository sits inside a `configure:template`
  section, so `configure` removes it and a configured package starts with an
  empty changelog rather than with this one's. It did not: the markers rename
  the template's values into the two files and nothing else, so two published
  packages shipped a changelog whose highest heading described a release of this
  repository, with everything they had actually added filed as unreleased.
- The release tests name no version of this repository. Two of them asserted
  that this repository's own notes were present, which a clone inherits and
  cannot satisfy -- so instead of catching the wrong content they held it in
  place. What is left is generic: an action declared in `policy.go` and a
  migration declared in `module.go` have to be named under a version heading
  rather than under `[Unreleased]`, the two release files have to describe the
  same set of versions, and no version may be headed twice. A package that has
  released nothing skips them.

### Added

- `TestTheReleaseHistoryBelongsToTheTemplateSection`, which refuses a release
  heading written outside the section `configure` removes.

### Fixed

- `## [0.3.1]` described what `v0.3.0` shipped, and `v0.3.0` had no entry at
  all. `v0.1.0` had none either, and `UPGRADE.md` had notes for neither those
  two nor `v0.3.1`.

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

### Fixed

- The changelog entry the release gate reads sits under the heading that names
  its version. The gate takes a release's notes out of that heading, and an
  entry still under `[Unreleased]` has none to be found by.

## [0.3.0] - 2026-09-03

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

## [0.1.0] - 2026-08-26

### Added

- The skeleton somebody clones to write an Arandu package: `Skeleton`,
  `SkeletonService`, `SkeletonPolicy`, `Module`, `Config`, and
  `20260823_0001_create_skeletons` written with the Blueprint.
- `SkeletonPolicy`, and the actions it answers about: `SkeletonView`,
  `SkeletonList`, `SkeletonCreate`, `SkeletonUpdate` and `SkeletonDelete`.
- `configure.go`, which turns the template into a package of its own and then
  deletes itself.
- The structure gate: a package arrives knowing what it has to look like, so a
  clone that drifts from the contract fails its own suite rather than somebody
  else's review.
- A route prefix a package cannot register is rejected rather than accepted and
  left unreachable.
- The skill that explains the package, and the vault note it records itself in,
  both cloned with it.
<!-- configure:template-end -->
