---
name: skeleton-package
description: Install, wire and use the :package_name package (Go, Arandu) in an application. Use when the request is to "install :package_name", "add :module_slug to the app", "go get :module_path", "wire it into bootstrap/app.go", "register the module", "use the :module_slug routes", "everything under /:module_slug returns 403", "403 forbidden from :module_slug", "it redirects to /auth/login", "the table does not exist", "no such table", "change where it is mounted", "let admins read it", or when a project's go.mod already requires :module_path. Covers the three lines of wiring and where each one goes, the Config fields, none of them required, the routes, their names and the session they need, why the policy refuses everything until somebody opens it, and the migration step that is not optional.
license: MIT
---

<!-- configure:template-start -->
This file is a template. It ships in the package skeleton with the placeholders
`configure.go` rewrites, so the package that comes out of the skeleton arrives
carrying the skill that teaches an assistant to install it. The directory name
is rewritten with the contents, because a skill whose frontmatter `name` and
directory disagree is a skill nothing loads.

Everything below is written as if the package existed. Keep it that way when you
edit it: the audience is somebody working in an *application* that installs the
package, not somebody changing the package.
<!-- configure:template-end -->
# Using :package_name

An Arandu package with one entity, its own table, its own routes and its own
policy. It is registered by hand — there is no service provider, no container
and no discovery, so if a line is not written in the application, it does not
happen.

## Install

```bash
go get :module_path
```

## The three lines of wiring, and where each one goes

All three are in `bootstrap/app.go`. Nothing else in the application changes,
and `aru make:module` does not edit this file for you.

The import, with the other module imports:

```go
import (
	:module_slug ":module_path"
)
```

The construction, in `Build`, after the session store exists and before
`k.Register`:

```go
	:module_slugModule, err := :module_slug.New(:module_slug.Config{}, db, sessions)
	if err != nil {
		return App{}, err
	}
```

And the registration, inside the `k.Register(...)` call already there:

```go
		:module_slugModule,
```

`New` returns an error rather than starting half-wired. It refuses a
configuration that cannot work, a nil database handle and a nil session store,
so a wiring mistake costs one restart instead of arriving as a nil dereference
on the first request that needed it.

## Then run the migrations

```bash
aru migrate
```

Not optional, and not automatic. The package owns a table, which is why its
`arandu.mod.toml` declares `migrations = true` — that declaration is how an
installer finds out before deploying rather than from the first request.

`aru migrate` is a step in the pipeline and never a call at the start of the
process: with N replicas, N migrations race each other.

**Symptom to recognise:** `no such table`, or `relation does not exist`, from a
route that authorized correctly. The migration has not run.

## Configuration

| field | required | meaning |
| --- | --- | --- |
| `Prefix` | no | where the routes are mounted. Defaults to `/:module_slug` |
| `PageSize` | no | how many records one page answers with. Defaults to 25, refused above 200 |

`:module_slug.Config{}` is a complete configuration. There is no tenant to pass:
every route needs a session, and the tenant of every statement comes from
`data.Tenant(g)`, off the Grant of whoever is signed in. Passing a value the
request named — a path segment, a header, a subdomain read off the URL — is what
`TestNoTenantIsReadOutOfTheRequest` fails on in the package. In an application
the same mistake is `aru doctor`'s `tenant-from-request`; in a package nothing
but the package's own suite looks.

`PageSize` above 200 is an error from `New`, not a silent 200. A number somebody
wrote and did not get is worse than a number somebody wrote and was told about.

## Routes

| method | path | name |
| --- | --- | --- |
| `GET` | `/:module_slug` | `:module_slug.index` |
| `GET` | `/:module_slug/{id}` | `:module_slug.show` |
| `POST` | `/:module_slug` | `:module_slug.store` |

Every route sits behind `RequireAuth`, mounted by the package itself. A request
with no session is sent to `/auth/login` — 303, or `HX-Redirect` for an htmx
request — and comes back to the address it asked for after signing in. That is
the sign-in screen the starter kit publishes; an application without one sees
a 404 there.

**Symptom to recognise:** every route of the package redirects to
`/auth/login`. Nobody is signed in on that request: the session cookie did not
arrive, or the session store passed to `New` is not the one the application
signs people in with.

Build URLs from the names, never by writing the path a second time. `aru
route:list` shows them grouped by module. Under a custom `Prefix` the paths move
and the names do not.

`GET /:module_slug` answers `{"data": {"items": [...]}}`, and adds
`"next_cursor"` beside `data` when a further page exists. Pass it back as
`?cursor=`. A page shorter than `PageSize` is the last one and carries no
cursor.

`TenantID` is not in any response. That is deliberate: it names a customer's
identifier and the response is a declared list of fields rather than the entity.

## Every route refuses everybody, until you open the policy

This is the state the package ships in, and it is not a bug to work around.
`SkeletonPolicy` denies every action and has no branch that allows one:

```
403 Forbidden
```

is what a correctly installed, correctly migrated, correctly wired package
answers to an administrator who is signed in, on day one.

Opening an action means writing the rule that opens it, in the package's
`policy.go`, inside the block that says so:

```go
	// arandu:begin custom
	if a == SkeletonView && (s.ID == record.ID || s.HasRole("admin")) {
		return nil
	}
	// arandu:end custom
```

What is not written there stays closed, including every action added later.
There are five actions — `SkeletonView`, `SkeletonList`, `SkeletonCreate`,
`SkeletonUpdate`, `SkeletonDelete` — and opening one opens exactly one.

**Do not open it from the application.** There is no hook, no override and no
config flag for the policy, and adding one would be a second place where
authorization is decided. If the rules this application needs are not the rules
the package ships, the change belongs in the package.

## What the answers mean

| status | what happened |
| --- | --- |
| `303` to `/auth/login` | there is no session on the request |
| `403` | the policy refused, or no rule allows the action yet |
| `404` | no row with that id in this tenant |
| `303` back to the page the request came from | the input was rejected; the messages and what was typed are in the flash |

The package does not choose these: its handlers return what the service
returned, and the framework's router answers it. A refusal carries no detail
beyond the status and its standard sentence. Telling a client why a policy said
no tells it what exists and what does not, one request at a time. The reason is
in the log.

Anything else is a 500 and the framework's error page in development. The
package returns unexpected errors rather than swallowing them, so a route that
answers 200 with an empty body is not something it does.

## Reporting a problem

Issues and pull requests go to the repository the module path names,
`:module_path`, which belongs to `:author_username`. A vulnerability goes to the
private advisory form named in that repository's `SECURITY.md`, and never into
an issue.

MIT licensed. Copyright :author_name.
