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

### The entity is a concrete type over the non-generic model

Hesape `v0.47.0` removed the generic model layer, and the package now requires
Hesape `v0.49.0` and Framework `v0.50.2`; `arandu.mod.toml` says
`framework = ">= 0.50"`. `Skeleton` embeds `model.Model`, its table is declared
once as `skeletonTable` in `model.go`, and `aru model:build` generates
`SkeletonQuery.go` beside it: `Skeletons`, `SkeletonQuery` and
`SkeletonCollection`. The file is committed, and `aru model:build --check` exits
1 when it is stale.

An application that uses the package through `New`, its routes and
`SkeletonService` changes nothing for this section: every service method keeps
its signature. What breaks is code that reached the table through `Skeletons`:

| before | now |
|---|---|
| `func Skeletons(*data.DB) *model.Model[Skeleton]` | `func Skeletons(model.DB) *SkeletonQuery`; a `*data.DB` is a `model.DB`, so the call compiles unchanged |
| `Skeletons(db).NewQuery().Where(…)` | `Skeletons(db).Where(…)` |
| `Skeletons(db).NewInstance(nil, false)`, then `.Entity` | `Skeletons(db).New()`, which returns the `*Skeleton` |
| `func(*model.Builder[Skeleton])` in a grouped where | `func(*SkeletonQuery)` |
| `record.Exists`, a field | `record.Exists()` |
| `record.Table`, a field | `record.Table()`, the `*model.Table` |
| the configuration the generic model carried as fields of `Skeleton` -- `ConnectionName`, `CreatedAtColumn`, `DeletedAtColumn`, `Entity`, `Grammar`, `Incrementing`, `KeyType`, `NamedScopes`, `PerPage`, `PrimaryKey`, `Processor`, `RelationResolvers`, `SoftDeletes`, `TenantColumn`, `Timestamps`, `UpdatedAtColumn`, `WasRecentlyCreated` | gone; the table settings live in `skeletonTable`, and a row keeps `Save`, `Delete`, `Fresh`, `Replicate`, `Exists`, `Table`, `WasRecentlyCreated()` and the attribute methods of `model.Model` |

Every one of those is `Skeleton.<field>` in the API diff: `Skeleton.ConnectionName`,
`Skeleton.CreatedAtColumn`, `Skeleton.DeletedAtColumn`, `Skeleton.Entity`,
`Skeleton.Exists`, `Skeleton.Grammar`, `Skeleton.Incrementing`,
`Skeleton.KeyType`, `Skeleton.NamedScopes`, `Skeleton.PerPage`,
`Skeleton.PrimaryKey`, `Skeleton.Processor`, `Skeleton.RelationResolvers`,
`Skeleton.SoftDeletes`, `Skeleton.Table`, `Skeleton.TenantColumn`,
`Skeleton.Timestamps`, `Skeleton.UpdatedAtColumn` and
`Skeleton.WasRecentlyCreated`.

**What changes without a compiler error.** A value copy of a `Skeleton` cannot
be saved: the write is refused with `model.ErrUnwired`, where before it acted on
the row the copy was taken from. And `Skeletons(db)` is now one mutable query
rather than a model that opened a new one per chain, so two queries begun from
one value held in a variable share their clauses: start each at the constructor.

To move a package already configured from this template:

1. `go get github.com/arandu-io/hesape@v0.49.0 github.com/arandu-io/framework@v0.50.2`,
   then `go mod tidy`, and raise the `framework` floor in `arandu.mod.toml` to
   `>= 0.50`.
2. `go run github.com/arandu-io/aru/cmd/model-upgrade@v0.60.5 --dry-run ./...`,
   then without `--dry-run`. It refuses a constructor held in a variable and
   used twice; start each query at the constructor and run it again.
3. `go run github.com/arandu-io/aru@v0.60.5 model:build`, and commit the
   generated `SkeletonQuery.go`.
4. Let the compiler name what is left; tests that read the model's settings
   observe what the table compiles instead.

<details>
<summary>Every method the generic model promoted on <code>*Skeleton</code> that the non-generic one does not, as the API diff names them</summary>

The five whose signature changed rather than disappeared:

- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Is`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).MakeHidden`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).MakeVisible`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetRelation`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SyncOriginal`

The rest are gone from the method set of `*Skeleton`:

- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).AddGlobalScope, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).All, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Append, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).AttributesToArray, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).CallNamedScope, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Create, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Destroy, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).DiscardChanges, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Except, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Find, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).FindMany, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).FindOrFail, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).FindOrNew, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).First, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).FirstOrCreate, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).FirstOrNew, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ForceCreate, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ForceDeleteQuietly, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ForceDeleted, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ForceDeleting, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ForceDestroy, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).FreshTimestamp, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetAppends, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetConnectionName, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetCreatedAtColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetDeletedAtColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetForeignKey, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetGlobalScopes, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetHidden, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetIncrementing, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetKeyName, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetKeyType, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetMorphClass, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetPerPage, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetPrevious, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQualifiedCreatedAtColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQualifiedDeletedAtColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQualifiedKeyName, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQualifiedUpdatedAtColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQueueableConnection, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQueueableID, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetQueueableRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetRawOriginal, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetRelation, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetRouteKey, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetRouteKeyName, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetTable, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetTouchedRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetUpdatedAtColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).GetVisible, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).HasAppended, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).HasGlobalScope, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).HasNamedScope, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).IsForceDeleting, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).IsIgnoringTouch, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).IsNot, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).IsRelation, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).IsSoftDeletable, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadAggregate, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorph, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorphAggregate, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorphAvg, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorphCount, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorphMax, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorphMin, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).LoadMorphSum, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewBaseQueryBuilder, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewCollection, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewFromBuilder, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewInstance, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewModelQuery, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewQuery, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewQueryForRestoration, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewQueryWithoutRelationships, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewQueryWithoutScope, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewQueryWithoutScopes, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).NewTypedBuilder, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).On, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).OnWriteConnection, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Only, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).OnlyTrashed, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).OriginalIsEquivalent, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).PushQuietly, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).QualifyColumn, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).QualifyColumns, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Query, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Ref, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).RegisterGlobalScopes, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).RegisterModelEvent, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ReplicateQuietly, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ResolveRouteBinding, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ResolveRouteBindingQuery, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ResolveSoftDeletableRouteBinding, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).RestoreQuietly, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Restored, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Restoring, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetAppends, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetConnection, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetHidden, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetIncrementing, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetKeyName, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetKeyType, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetPerPage, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetTable, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetTouchedRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SetVisible, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SoftDeleted, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SyncChanges, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SyncOriginalAttribute, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).SyncOriginalAttributes, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).ToPrettyJSON, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Touches, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UnsetAttribute, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UnsetRelation, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UnsetRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UpdateOrCreate, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UpdateOrFail, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UpdateQuietly, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UpdateTimestamps, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).UsesTimestamps, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).Where, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).WhereKey, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).With, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).WithTrashed, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).WithoutRelations, method set of *Skeleton`
- `github.com/arandu-io/hesape/database/model.(*Model[github.com/arandu-io/package-skeleton.Skeleton]).WithoutTimestamps, method set of *Skeleton`

</details>

### Routes need a session, and the router answers the errors

Every route is registered behind `RequireAuth`, and each handler reads who is
asking with `ctx.User()` and returns what the service returned. The module no
longer loads the session itself, reads no guest, and maps no error to a status.

`Config.Tenant` was removed. It was the tenant a visitor with no session was
read as, and with every route behind the guard there is no such visitor:

```go
// Before.
skeleton.New(skeleton.Config{Tenant: cfg.Auth.Tenant}, db, sessions)

// After.
skeleton.New(skeleton.Config{}, db, sessions)
```

What a client sees changes with it, and none of it is a compiler error:

| request | before | now |
|---|---|---|
| no session | 403, after the policy refused a guest | 303 to `/auth/login`, before any handler runs |
| input rejected | 422, with the field names in the body | 303 back to the referring page, with the messages and what was typed in the flash |
| policy refused | 403 `forbidden` | 403 `Forbidden`, the standard sentence |
| no such record | 404 `not found` | 404 `Not Found` |

`ErrNotFound` now wraps `model.ErrModelNotFound`, which is what makes the router
answer it with 404. `errors.Is(err, ErrNotFound)` still holds; its text is
longer.

To move a package already configured from this template: replace each
handler's `m.subject(ctx.Request)` with `who, _ := ctx.User()` and each
`return m.answer(ctx, err)` with `return err`; delete `subject` and `answer`;
open `Routes` with `r = r.Group("", middleware.RequireAuth(m.sessions))`; make
`ErrNotFound` wrap `model.ErrModelNotFound`; and remove `Tenant` from `Config`
and from the wiring in `bootstrap/app.go`.

A client that cannot follow a redirect no longer reads the rejected field names
from the body. A module that has to answer such a client 422 returns an error
type of its own with an `HTTPStatus() int` method, which the router answers with
that status and its standard sentence -- still without the field names, which go
to the flash or nowhere.

<!-- configure:template-start -->
The notes below are the release history of the package skeleton this file was
cloned with. `configure` removes them.

## v0.6.2

No API or database change. Newly configured packages use Framework v0.47.1 and Hesape v0.41.1. Existing applications update those requirements normally. The v0.6.1 tag remains immutable; this release supplies the versioned publication metadata it lacked.

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
