# Changelog

Everything worth knowing about a release of :package_name is recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
the versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

A published module version is immutable: Go serves it from the proxy forever, so
a release is corrected by another release and never by moving a tag.

## [Unreleased]

### Changed

- Generic CRUD now uses the configured `Skeletons(db)` Model and its Builder.
- `NewSkeletonService` now accepts `*data.DB` directly.
- `SkeletonService.Create`, `Find`, and `List` now return Model-backed pointers;
  keep those pointers intact until taking a response snapshot.
- `Skeleton` now embeds `model.Model[Skeleton]` and is no longer comparable.

### Removed

- `SkeletonRepository`, `NewSkeletonRepository`, and the five generic CRUD
  methods. Add a Repository only for specialized queries, reports, projections,
  read models, exports, or external storage.
