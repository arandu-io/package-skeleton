# Upgrade Guide

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
