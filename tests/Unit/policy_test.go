package unit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/arandu-io/framework/data"
	"github.com/arandu-io/framework/security"
	"github.com/arandu-io/hesape/database/model"
	"github.com/arandu-io/hesape/database/query"

	skeleton "github.com/arandu-io/package-skeleton"
)

// The four properties this package exists to keep are checked here, and they
// are checked against the code rather than described in a document:
//
//  1. the policy denies every action, and has no branch that allows one;
//  2. the service authorizes before constructing or running the generated query;
//  3. the tenant comes from the Grant;
//  4. nothing reaches the database without passing through the first two.
//
// The fourth is checked by the handle these tests pass in. It wraps a nil
// *sql.DB, so any statement that were issued would panic and fail the test
// loudly -- which makes "the refusal happened before the Model" a fact the
// suite proves rather than a comment. The structural twin in audit_test.go
// keeps that order visible on every service method, including an allowed path.

// everyAction is the whole set the policy answers about. A test that listed
// four of five would pass while the fifth was open.
var everyAction = []security.Action{
	skeleton.SkeletonView,
	skeleton.SkeletonList,
	skeleton.SkeletonCreate,
	skeleton.SkeletonUpdate,
	skeleton.SkeletonDelete,
}

// administrator is the most privileged subject an application can produce. It
// is the one to test the default with: a policy that refuses an administrator
// refuses everyone.
func administrator() security.Subject {
	return security.Subject{ID: "user-1", Tenant: "acme", Roles: []string{"admin"}, Verified: true}
}

func TestThePolicyDeniesEveryActionByDefault(t *testing.T) {
	t.Parallel()

	for _, action := range everyAction {
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			_, err := security.Authorize(context.Background(), skeleton.SkeletonPolicy{},
				administrator(), action, skeleton.Skeleton{})
			if !errors.Is(err, security.ErrForbidden) {
				t.Fatalf("an unopened policy allowed %s: got %v, want ErrForbidden", action, err)
			}
		})
	}
}

func TestThePolicyDeniesARecordOfAnotherTenant(t *testing.T) {
	t.Parallel()

	other := skeleton.Skeleton{ID: "record-1", TenantID: "globex", Name: "theirs"}

	err := skeleton.SkeletonPolicy{}.Can(context.Background(),
		administrator(), skeleton.SkeletonView, other)
	if err == nil {
		t.Fatal("the policy allowed a record belonging to another tenant")
	}
	// The message is asserted because the tenant check is the one refusal that
	// has to survive somebody opening the actions below it.
	if !strings.Contains(err.Error(), "another tenant") {
		t.Fatalf("the refusal did not name the tenant: %v", err)
	}
}

func TestThePolicyDeniesAGuest(t *testing.T) {
	t.Parallel()

	for _, action := range everyAction {
		_, err := security.Authorize(context.Background(), skeleton.SkeletonPolicy{},
			security.Guest("acme"), action, skeleton.Skeleton{})
		if !errors.Is(err, security.ErrForbidden) {
			t.Fatalf("a guest was allowed %s: got %v, want ErrForbidden", action, err)
		}
	}
}

func TestAuthorizeRefusesASubjectThatIsNobody(t *testing.T) {
	t.Parallel()

	// The zero Subject is a session that failed to load, not an anonymous
	// reader, and it is refused before the policy is consulted. A package that
	// answered it as a guest would answer a broken session as a visitor.
	_, err := security.Authorize(context.Background(), skeleton.SkeletonPolicy{},
		security.Subject{}, skeleton.SkeletonView, skeleton.Skeleton{})
	if !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("an empty subject was authorized: got %v, want ErrForbidden", err)
	}
}

// nilHandle is a handle over no database.
//
// Any statement issued through it panics, which is what makes these tests
// prove that the refusal came first: a service that reached the Model before
// authorizing would crash here rather than pass.
func nilHandle() *data.DB { return data.Wrap(nil, data.DialectSQLite) }

func TestTheServiceRefusesBeforeReachingTheModel(t *testing.T) {
	t.Parallel()

	// A nil handle makes even construction of Skeletons panic, at the
	// GetQueryGrammar the query asks the handle for. This catches moving the
	// generated query constructor -- not only its terminal -- ahead of
	// authorization.
	service := skeleton.NewSkeletonService(nil)
	ctx := context.Background()

	if _, err := service.Find(ctx, administrator(), "record-1"); !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("Find reached the Model before the policy refusal: %v", err)
	}
	if _, err := service.List(ctx, administrator(), data.Query{}); !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("List reached the Model before the policy refusal: %v", err)
	}
	if _, err := service.Create(ctx, administrator(), skeleton.CreateRequest{Name: "one"}); !errors.Is(err, security.ErrForbidden) {
		t.Fatalf("Create reached the Model before the policy refusal: %v", err)
	}
}

// recordingHandle runs nothing and keeps every statement it is handed, so a
// test reads what the table compiles instead of reading its settings. The table
// keeps its settings to itself; what it writes is the behaviour they decide.
type recordingHandle struct {
	*data.DB
	statements []recordedStatement
}

type recordedStatement struct {
	sql      string
	bindings []any
}

// newRecordingHandle borrows the grammar and the processor of an SQLite handle
// over no database, and answers every statement itself.
func newRecordingHandle() *recordingHandle { return &recordingHandle{DB: nilHandle()} }

func (h *recordingHandle) record(sql string, bindings []any) {
	h.statements = append(h.statements, recordedStatement{sql: sql, bindings: slices.Clone(bindings)})
}

func (h *recordingHandle) Select(_ context.Context, sql string, bindings []any, _ bool) ([]query.Record, error) {
	h.record(sql, bindings)
	return nil, nil
}

func (h *recordingHandle) Insert(_ context.Context, sql string, bindings []any) (bool, error) {
	h.record(sql, bindings)
	return true, nil
}

func (h *recordingHandle) Update(_ context.Context, sql string, bindings []any) (int64, error) {
	h.record(sql, bindings)
	return 1, nil
}

func (h *recordingHandle) Delete(_ context.Context, sql string, bindings []any) (int64, error) {
	h.record(sql, bindings)
	return 1, nil
}

func (h *recordingHandle) Statement(_ context.Context, sql string, bindings []any) (bool, error) {
	h.record(sql, bindings)
	return true, nil
}

// first is the first recorded statement that starts with verb.
func (h *recordingHandle) first(t *testing.T, verb string) recordedStatement {
	t.Helper()
	for _, statement := range h.statements {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(statement.sql)), verb) {
			return statement
		}
	}
	t.Fatalf("no %s statement was issued; recorded %v", verb, h.statements)
	return recordedStatement{}
}

// TestSkeletonsReturnsAWiredTenantScopedQuery holds what skeletonTable declares,
// through what it makes the generated query do: the table it reads, a key the
// application writes and the engine never fills, and a tenant_id taken from the
// Grant on the way in and on the way out.
func TestSkeletonsReturnsAWiredTenantScopedQuery(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	handle := newRecordingHandle()
	table := skeleton.Skeletons(handle).Base().Table()
	if table.Name() != "skeletons" {
		t.Fatalf("Skeletons table = %q, want skeletons", table.Name())
	}
	if key := table.MorphModel(handle); key.GetKeyName() != "id" || key.GetKeyType() != "string" {
		t.Fatalf("Skeletons key is %q of type %q; want id, as text", key.GetKeyName(), key.GetKeyType())
	}

	row, err := skeleton.Skeletons(handle).New()
	if err != nil {
		t.Fatalf("building a row: %v", err)
	}
	if row.Table() != table {
		t.Fatal("Skeletons returned an entity whose embedded Model is not wired to its table")
	}

	// The key is application-generated text: the insert carries the one the row
	// was given, and the engine is never asked for one. An incrementing key
	// would go through the processor, which runs on no database here, and a key
	// the model generated would replace the one the service wrote.
	row.ID = "record-1"
	if _, err := row.Save(ctx, security.SystemGrant(skeleton.SkeletonCreate, "acme")); err != nil {
		t.Fatalf("saving through the recording handle: %v", err)
	}
	insert := handle.first(t, "insert")
	if !slices.Contains(insert.bindings, any("record-1")) || row.ID != "record-1" {
		t.Fatalf("the insert %q %v does not write the application key, or the row lost it (%q)", insert.sql, insert.bindings, row.ID)
	}
	if !strings.Contains(insert.sql, "tenant_id") || !slices.Contains(insert.bindings, any("acme")) {
		t.Fatalf("the insert %q %v does not stamp tenant_id with the Grant's tenant", insert.sql, insert.bindings)
	}

	// And the tenant column scopes a read by the Grant's tenant.
	if _, err := skeleton.Skeletons(handle).WhereKey("record-1").First(ctx, security.SystemGrant(skeleton.SkeletonView, "acme")); err != nil {
		t.Fatalf("reading through the recording handle: %v", err)
	}
	read := handle.first(t, "select")
	if !strings.Contains(read.sql, "tenant_id") || !slices.Contains(read.bindings, any("acme")) {
		t.Fatalf("the read %q %v is not filtered by tenant_id with the Grant's tenant", read.sql, read.bindings)
	}
}

// TestTheMissingRecordIsTheModelsNotFound holds the one link between this
// package's sentinel and the router's answer. The router answers
// model.ErrModelNotFound with 404 and has never heard of ErrNotFound, so the
// sentinel is a 404 only while it wraps the model's.
func TestTheMissingRecordIsTheModelsNotFound(t *testing.T) {
	t.Parallel()

	if !errors.Is(skeleton.ErrNotFound, model.ErrModelNotFound) {
		t.Fatal("ErrNotFound does not wrap model.ErrModelNotFound, so the router would answer a missing record with 500")
	}
}

func TestASystemGrantWithoutATenantReachesNothing(t *testing.T) {
	t.Parallel()

	// A system grant with no tenant names no customer. The Model refuses it
	// while preparing the query, before the nil handle can issue a statement.
	_, err := skeleton.Skeletons(nilHandle()).WhereKey("record-1").First(
		context.Background(), security.SystemGrant(skeleton.SkeletonView, ""))
	if !errors.Is(err, model.ErrNoTenant) {
		t.Fatalf("a system grant with no tenant returned %v, want ErrNoTenant", err)
	}
}

func TestTheTenantComesFromTheGrant(t *testing.T) {
	t.Parallel()

	g := security.SystemGrant(skeleton.SkeletonView, "acme")
	if got := data.Tenant(g); got != "acme" {
		t.Fatalf("data.Tenant(g) = %q, want %q", got, "acme")
	}

	// And a Grant nobody issued carries no tenant at all, so a statement that
	// took its tenant from anywhere else would be reading rows this Grant does
	// not name.
	if got := data.Tenant(security.Grant{}); got != "" {
		t.Fatalf("the zero Grant carries the tenant %q, want none", got)
	}
}

func TestTheRequestValidatesItsInput(t *testing.T) {
	t.Parallel()

	if errs := (skeleton.CreateRequest{}).Validate(); !errs.Any() {
		t.Fatal("an empty request validated")
	}
	if errs := (skeleton.CreateRequest{Name: strings.Repeat("a", 121)}).Validate(); !errs.Any() {
		t.Fatal("a name past the maximum validated")
	}
	if errs := (skeleton.CreateRequest{Name: "one"}).Validate(); errs.Any() {
		t.Fatalf("a valid request was rejected: %v", errs)
	}
}

func TestTheConfigurationRefusesWhatCannotWork(t *testing.T) {
	t.Parallel()

	for name, cfg := range map[string]skeleton.Config{
		"relative prefix":    {Prefix: "skeleton"},
		"page size too big":  {PageSize: skeleton.MaxPageSize + 1},
		"negative page size": {PageSize: -1},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("the configuration with %s was accepted", name)
		}
	}

	// The zero value is the configuration an application that changes nothing
	// writes, and it has to be one New accepts: there is no tenant to name,
	// because every tenant comes from the Grant.
	if err := (skeleton.Config{}).Validate(); err != nil {
		t.Fatalf("the zero configuration was refused: %v", err)
	}
}
