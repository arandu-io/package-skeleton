package skeleton

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
)

// ErrNotFound is returned when no row matches, so callers do not have to import
// database/sql to compare against sql.ErrNoRows.
var ErrNotFound = errors.New("skeleton: record not found")

// Pagination bounds for List. A request that asks for everything gets the
// maximum, never everything: an unbounded query is how one page load takes a
// production database down.
const (
	defaultLimit = 50
	maxLimit     = 200
)

// SkeletonRepository is the only door to the skeletons table.
//
// It is a hand-written CRUD repository: five methods over one table, with the
// four statements written out. That is what this package starts from, and it is
// not where the data path ends up. Trivial CRUD belongs to the framework's
// generic model and the query builder it carries, which take the same Grant on
// every read and every write and derive the same four statements from the
// entity. What is left to a repository after that is the work no generated
// statement covers: a query that joins two aggregates, a read model, a report,
// an export.
//
// The statements are written out here rather than taken from that model because
// the model's signatures are still moving, and a package built against a moving
// signature is a package that stops compiling for whoever installed it. When
// they settle, this file is the one to replace. The three properties below are
// what has to survive that replacement, and the model keeps all three -- so
// what changes is how the statements are spelled, and nothing above them.
//
// Every method takes the Grant before the id, and the order is the mechanism
// rather than a convention. A caller cannot name a record without first holding
// a decision that was already made about it, and cannot leave the decision out,
// because the call does not compile without it.
//
// Every method opens with g.Check, which proves the Grant was issued for this
// exact action. That is the second half: a Grant obtained for one action and
// spent on another is the copy-paste between two repository methods, and it
// fails here rather than reaching a row.
//
// The tenant is read from the Grant with data.Tenant and from nowhere else --
// never from the path, the body, the query string or a header. The value on the
// Grant came from the session; a value that came in with the request is a value
// the caller chose.
//
// The SQL is written with "?" placeholders and with types every supported
// engine shares, so the same statements run on SQLite and on PostgreSQL. The
// dialect rebinds the placeholders; nothing else needs translating. This
// paragraph is the one the model makes unnecessary: the placeholders and the
// portable types are what it emits from the entity, and the three properties
// above are what it does not emit and cannot replace.
type SkeletonRepository struct {
	db *data.DB
}

// NewSkeletonRepository returns a repository over an instrumented handle.
//
// The handle is the framework's wrapper and not a bare *sql.DB, which is what
// puts every statement issued here on the debug page with the file and line
// that issued it.
func NewSkeletonRepository(db *data.DB) *SkeletonRepository {
	return &SkeletonRepository{db: db}
}

// Repository conformance is asserted at compile time: if the contract changes,
// this line breaks before any caller does.
var _ data.Repository[Skeleton, string] = (*SkeletonRepository)(nil)

// skeletonColumns is the column list, written once. Two spellings of it drift,
// and the way they drift is a scan that reads the wrong column into the wrong
// field without failing.
const skeletonColumns = `id, tenant_id, name, created_at`

// Find returns one record by id, scoped to the grant's tenant.
func (r *SkeletonRepository) Find(ctx context.Context, g security.Grant, id string) (Skeleton, error) {
	if err := g.Check(SkeletonView); err != nil {
		return Skeleton{}, err
	}
	row := r.db.QueryRowContext(ctx,
		`SELECT `+skeletonColumns+` FROM skeletons WHERE id = ? AND tenant_id = ?`,
		id, data.Tenant(g))
	return scanSkeleton(row)
}

// sortableSkeleton is the ordering allowlist.
//
// A sort field is a column name, and a column name interpolated from the
// request is injection through another door -- so the only accepted values are
// the ones listed here, and anything else is refused rather than ignored.
var sortableSkeleton = map[string]string{
	"":           "created_at",
	"name":       "name",
	"created_at": "created_at",
}

// List returns a page of records in the grant's tenant.
//
// Reading is authorized like writing. There is no listing that skips the
// policy: a read model with no check is a leak between customers with a
// technical name.
//
// Pagination is keyset based on (created_at, id). OFFSET grows more expensive
// with every page and skips rows when data changes underneath it.
func (r *SkeletonRepository) List(ctx context.Context, g security.Grant, q data.Query) ([]Skeleton, error) {
	if err := g.Check(SkeletonList); err != nil {
		return nil, err
	}
	column, ok := sortableSkeleton[q.Sort]
	if !ok {
		return nil, fmt.Errorf("skeleton: sort field not allowed: %q", q.Sort)
	}
	limit := q.Limit
	switch {
	case limit <= 0:
		limit = defaultLimit
	case limit > maxLimit:
		limit = maxLimit
	}

	query := `SELECT ` + skeletonColumns + ` FROM skeletons WHERE tenant_id = ?`
	args := []any{data.Tenant(g)}
	if q.Cursor != "" {
		// Row values -- (a, b) > (c, d) -- are not portable, so the comparison
		// is written out. It is the same index scan, spelled for every engine.
		query += ` AND (created_at > (SELECT created_at FROM skeletons WHERE id = ? AND tenant_id = ?)
		            OR (created_at = (SELECT created_at FROM skeletons WHERE id = ? AND tenant_id = ?) AND id > ?))`
		args = append(args, q.Cursor, args[0], q.Cursor, args[0], q.Cursor)
	}
	query += ` ORDER BY ` + column + `, id LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Skeleton
	for rows.Next() {
		record, err := scanSkeleton(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

// Create inserts the record and returns it as stored.
//
// The tenant on the entity is overwritten with the one from the Grant rather
// than trusted. A caller that filled the field in has filled in a guess; the
// Grant carries the answer, and writing the guess is how a row lands in another
// customer's data with every check having passed.
//
// The id and the timestamp are generated here rather than by the database, for
// the reason the entity gives: a DEFAULT that produces a uuid is spelled
// differently in every engine, and generating them in Go is what keeps one
// schema working everywhere.
func (r *SkeletonRepository) Create(ctx context.Context, g security.Grant, record Skeleton) (Skeleton, error) {
	if err := g.Check(SkeletonCreate); err != nil {
		return Skeleton{}, err
	}
	if record.ID == "" {
		var err error
		if record.ID, err = data.NewID(); err != nil {
			return Skeleton{}, err
		}
	}
	record.TenantID = data.Tenant(g)
	record.CreatedAt = time.Now().UTC()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO skeletons (`+skeletonColumns+`) VALUES (?, ?, ?, ?)`,
		record.ID, record.TenantID, record.Name, record.CreatedAt)
	if err != nil {
		return Skeleton{}, err
	}
	return record, nil
}

// Update writes the mutable fields.
//
// The tenant is not one of them: moving a record between customers is not an
// update, and a statement that could do it by accident is a statement that
// eventually does.
//
// A statement that changed no row is ErrNotFound rather than silence. An update
// that reported success and wrote nothing is somebody typing a change and
// reading the old value back.
func (r *SkeletonRepository) Update(ctx context.Context, g security.Grant, record Skeleton) (Skeleton, error) {
	if err := g.Check(SkeletonUpdate); err != nil {
		return Skeleton{}, err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE skeletons SET name = ? WHERE id = ? AND tenant_id = ?`,
		record.Name, record.ID, data.Tenant(g))
	if err != nil {
		return Skeleton{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Skeleton{}, err
	}
	if n == 0 {
		return Skeleton{}, ErrNotFound
	}
	record.TenantID = data.Tenant(g)
	return record, nil
}

// Delete removes one record within the grant's tenant.
func (r *SkeletonRepository) Delete(ctx context.Context, g security.Grant, id string) error {
	if err := g.Check(SkeletonDelete); err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM skeletons WHERE id = ? AND tenant_id = ?`, id, data.Tenant(g))
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows, so one scan function
// serves the single-row and the multi-row query.
type rowScanner interface{ Scan(dest ...any) error }

// scanSkeleton reads one row, and translates the absence of one into an error
// this package owns.
func scanSkeleton(row rowScanner) (Skeleton, error) {
	var record Skeleton
	err := row.Scan(&record.ID, &record.TenantID, &record.Name, &record.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Skeleton{}, ErrNotFound
	}
	if err != nil {
		return Skeleton{}, err
	}
	record.CreatedAt = record.CreatedAt.UTC()
	return record, nil
}
