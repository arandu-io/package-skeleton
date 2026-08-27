package unit_test

import (
	"go/ast"
	"go/build/constraint"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What this package proves about itself, before anybody installs it.
//
// `aru doctor` audits the application it is run inside. It walks that project's
// own tree, skips vendor/, reads the one arandu.mod.toml at its root, and
// refuses a directory that is not an application at all. It never loads a
// dependency. So nothing an installed package does is audited by the
// application that installed it: a package that reached a table without a
// Grant, took a tenant out of a request, or declared no capabilities and then
// opened a socket would pass every check the installer runs.
//
// The package is therefore the only place that check can happen, and this is
// it. These tests read this package's own Go files as syntax and hold four
// properties:
//
//  1. every function that issues a statement takes a Grant, takes it first, and
//     checks it before the statement;
//  2. the tenant comes from the Grant, and nothing a request carried is read as
//     one;
//  3. nothing reaches a repository without the policy having answered;
//  4. what arandu.mod.toml declares is what the code does.
//
// The fifth is held where the routes exist rather than here, because a prefix
// arrives through configuration and syntax cannot follow it:
// TestNoRouteLandsInTheFrameworkNamespace, in tests/Feature/routes_test.go,
// registers the module and reads the table back.
//
// What these tests do not reach is worth as much as what they do. They read
// syntax: a call made through a variable of interface type, a statement
// assembled out of a package-level variable, and anything reached by reflection
// are all invisible to them. A green run means no such thing was found written
// down, not that none exists. What is absolute is what the compiler holds
// alongside them -- a method with no Grant in its signature does not acquire one
// at runtime.

// buildable reports whether the compiler ever reads this file.
//
// It asks the build constraint rather than the file name, so a file excluded by
// a tag is left out of the audit whatever it is called: what the compiler never
// builds is not part of what the package does, and reporting it as such would
// be reporting a capability nobody can reach.
func buildable(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, comment := range group.List {
			expression, err := constraint.Parse(comment.Text)
			if err != nil {
				continue
			}
			// No tag set, which is what the gates run with.
			if !expression.Eval(func(string) bool { return false }) {
				return false
			}
		}
	}
	return true
}

// auditedFiles is every Go file the compiler builds into this package.
func auditedFiles(t *testing.T) []parsedGoFile {
	t.Helper()

	out := []parsedGoFile{}
	for _, source := range productionGoFiles(t, packageRoot(t)) {
		if buildable(source.file) {
			out = append(out, source)
		}
	}
	// An audit with nothing to read passes, and a test that passes by finding
	// nothing is the failure mode of every rule below.
	if len(out) == 0 {
		t.Fatal("no buildable Go file was found, so everything below would pass by having nothing to read")
	}
	return out
}

// handleCalls are the calls that put a statement on the wire under a name that
// cannot mean anything else.
//
// The short spellings -- Query, Exec -- are deliberately absent: ctx.Query
// reads a query string, and a rule that cannot tell the two apart is a rule an
// author switches off. What they would have caught is caught by the SQL
// instead, which is the more honest signal anyway.
var handleCalls = map[string]bool{
	"QueryContext":    true,
	"QueryRowContext": true,
	"ExecContext":     true,
	"BeginTx":         true,
	"Transaction":     true,
}

// reachesData reports whether a call issues a statement, either by the name of
// a handle method or by carrying SQL.
func reachesData(call *ast.CallExpr) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok && handleCalls[selector.Sel.Name] {
		return true
	}
	for _, argument := range call.Args {
		if carriesSQL(argument) {
			return true
		}
	}
	return false
}

// carriesSQL reports whether an expression opens with a statement keyword.
//
// It reads the leftmost literal of a concatenation, which is where the verb
// is. A column list held in a constant is joined onto "SELECT " and never
// starts one, so following the left edge is what makes a built statement
// readable without evaluating it.
func carriesSQL(expression ast.Expr) bool {
	for {
		binary, ok := expression.(*ast.BinaryExpr)
		if !ok || binary.Op != token.ADD {
			break
		}
		expression = binary.X
	}
	literal, ok := expression.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}

	text := strings.ToLower(strings.TrimSpace(strings.Trim(literal.Value, "`\"")))
	for _, verb := range []string{"select ", "insert ", "update ", "delete ", "with "} {
		if strings.HasPrefix(text, verb) {
			return true
		}
	}
	return false
}

// firstReach is where a body issues its first statement, and whether it issues
// one at all.
func firstReach(body *ast.BlockStmt) (token.Pos, bool) {
	found := token.NoPos
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !reachesData(call) {
			return true
		}
		if found == token.NoPos || call.Pos() < found {
			found = call.Pos()
		}
		return true
	})
	return found, found != token.NoPos
}

// grantParameter is the name of the Grant a function takes and where it sits in
// the signature.
func grantParameter(function *ast.FuncDecl) (name string, position int, found bool) {
	index := 0
	for _, field := range function.Type.Params.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{{Name: "_"}}
		}
		for _, identifier := range names {
			if namedType(field.Type) == "Grant" {
				return identifier.Name, index, true
			}
			index++
		}
	}
	return "", 0, false
}

// contextFirst reports whether the signature opens with a context, which is the
// one parameter allowed to precede the Grant.
func contextFirst(function *ast.FuncDecl) bool {
	parameters := function.Type.Params.List
	return len(parameters) > 0 && namedType(parameters[0].Type) == "Context"
}

// checkCalls returns where the named Grant is checked, and where the answer is
// thrown away.
func checkCalls(body *ast.BlockStmt, grant string) (asked, discarded []token.Pos) {
	isCheck := func(node ast.Node) (*ast.CallExpr, bool) {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return nil, false
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Check" {
			return nil, false
		}
		receiver, ok := selector.X.(*ast.Ident)
		return call, ok && receiver.Name == grant
	}

	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for _, value := range node.Rhs {
				if call, ok := isCheck(value); ok && allBlank(node.Lhs) {
					discarded = append(discarded, call.Pos())
				}
			}
		case *ast.ExprStmt:
			// A bare call is the same discard written shorter.
			if call, ok := isCheck(node.X); ok {
				discarded = append(discarded, call.Pos())
			}
		}
		if call, ok := isCheck(node); ok {
			asked = append(asked, call.Pos())
		}
		return true
	})
	return asked, discarded
}

// allBlank reports whether every target of an assignment is the blank
// identifier.
func allBlank(targets []ast.Expr) bool {
	for _, target := range targets {
		identifier, ok := target.(*ast.Ident)
		if !ok || identifier.Name != "_" {
			return false
		}
	}
	return len(targets) > 0
}

// earliest is the first of a set of positions inside one file.
func earliest(positions []token.Pos) token.Pos {
	first := token.NoPos
	for _, position := range positions {
		if first == token.NoPos || position < first {
			first = position
		}
	}
	return first
}

// selectorName is the trailing name of a selector, or the name of an
// identifier.
func selectorName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.Ident:
		return expression.Name
	}
	return ""
}

// calledName is the trailing name of whatever a call names, so a call can be
// recognised without resolving what it is called on.
func calledName(call *ast.CallExpr) string {
	return selectorName(call.Fun)
}

// firstCallTo is where a body first calls something by this name.
func firstCallTo(body *ast.BlockStmt, name string) token.Pos {
	found := token.NoPos
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || calledName(call) != name {
			return true
		}
		if found == token.NoPos || call.Pos() < found {
			found = call.Pos()
		}
		return true
	})
	return found
}

// TestEveryStatementTakesAGrantAndChecksItFirst holds the property the whole
// package is shaped around, on every function that issues a statement rather
// than on the five that exist today.
//
// The order is part of it. A Grant that arrives after the identifier is a
// signature a caller can name a record in before holding any decision about it,
// and the compiler is what refuses that -- but only while the parameter is
// where it belongs.
func TestEveryStatementTakesAGrantAndChecksItFirst(t *testing.T) {
	t.Parallel()

	audited := 0
	for _, source := range auditedFiles(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			reach, reaches := firstReach(function.Body)
			if !reaches {
				continue
			}
			audited++

			grant, position, takesGrant := grantParameter(function)
			if !takesGrant {
				t.Errorf("%s: %s issues a statement and takes no Grant, so nothing was decided before a row was read",
					source.path, function.Name.Name)
				continue
			}
			if position > 1 || (position == 1 && !contextFirst(function)) {
				t.Errorf("%s: %s takes the Grant at position %d; it goes first, or straight after the context",
					source.path, function.Name.Name, position)
			}

			asked, discarded := checkCalls(function.Body, grant)
			if len(discarded) > 0 {
				t.Errorf("%s: %s throws away the answer of %s.Check, which is the same as never asking",
					source.path, function.Name.Name, grant)
			}
			if len(asked) == 0 {
				t.Errorf("%s: %s holds a Grant and never calls %s.Check, so a Grant issued for another action reaches this statement",
					source.path, function.Name.Name, grant)
				continue
			}
			if earliest(asked) > reach {
				t.Errorf("%s: %s checks the Grant after it has already issued a statement",
					source.path, function.Name.Name)
			}
		}
	}
	if audited == 0 {
		t.Fatal("no function in this package issues a statement, so this test proved nothing")
	}
}

// requestAccessors are the ways a value that arrived with the request is read.
// A tenant taken through any of them is a tenant the caller chose.
var requestAccessors = map[string]bool{
	"Param":         true,
	"Query":         true,
	"Input":         true,
	"FormValue":     true,
	"PostFormValue": true,
	"PathValue":     true,
	"Cookie":        true,
	"Get":           true,
}

// TestNoTenantIsReadOutOfTheRequest is the rule with no exception in it. A
// tenant the client can name is a client who chooses whose rows they read, and
// every other check in the package passes while it happens.
func TestNoTenantIsReadOutOfTheRequest(t *testing.T) {
	t.Parallel()

	for _, source := range auditedFiles(t) {
		ast.Inspect(source.file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !requestAccessors[selector.Sel.Name] {
				return true
			}
			for _, argument := range call.Args {
				literal, ok := argument.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				if strings.Contains(strings.ToLower(literal.Value), "tenant") {
					t.Errorf("%s: %s(%s) reads a tenant out of the request; it comes from the Grant, which came from the session",
						source.path, selector.Sel.Name, literal.Value)
				}
			}
			return true
		})
	}
}

// TestEveryStatementTakesItsTenantFromTheGrant is the other half, and it is the
// half that catches the omission rather than the mistake: a statement that
// names no tenant at all reads every customer's rows and refuses nobody.
func TestEveryStatementTakesItsTenantFromTheGrant(t *testing.T) {
	t.Parallel()

	audited := 0
	for _, source := range auditedFiles(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			if _, reaches := firstReach(function.Body); !reaches {
				continue
			}
			audited++

			if firstCallTo(function.Body, "Tenant") == token.NoPos {
				t.Errorf("%s: %s issues a statement and never asks the Grant for the tenant, so the rows it names were not narrowed to one customer",
					source.path, function.Name.Name)
			}
			for _, written := range tenantWrites(function.Body) {
				t.Errorf("%s: %s writes a tenant field from %s; where a statement is issued, the only source is the Grant",
					source.path, function.Name.Name, written)
			}
		}
	}
	if audited == 0 {
		t.Fatal("no function in this package issues a statement, so this test proved nothing")
	}
}

// tenantWrites returns every value written into a tenant field that did not
// come from the Grant.
//
// It only looks inside a body that issues a statement. Elsewhere a tenant is
// legitimately taken from the subject -- building the candidate a policy
// decides about is exactly that -- and a rule that could not tell the two apart
// would reject the code it exists to protect.
func tenantWrites(body *ast.BlockStmt) []string {
	fromGrant := func(value ast.Expr) bool {
		call, ok := value.(*ast.CallExpr)
		return ok && calledName(call) == "Tenant"
	}

	var out []string
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for i, target := range node.Lhs {
				if i >= len(node.Rhs) || !strings.Contains(selectorName(target), "Tenant") {
					continue
				}
				if !fromGrant(node.Rhs[i]) {
					out = append(out, types.ExprString(node.Rhs[i]))
				}
			}
		case *ast.KeyValueExpr:
			if strings.Contains(selectorName(node.Key), "Tenant") && !fromGrant(node.Value) {
				out = append(out, types.ExprString(node.Value))
			}
		}
		return true
	})
	return out
}

// TestNothingReachesARepositoryWithoutThePolicy closes the loop the other tests
// leave open. The repository refuses a Grant issued for another action, and the
// policy is what issues one at all -- so a caller that built a Grant some other
// way, or reached the repository before deciding anything, satisfies every
// check and skips the only authority there is.
func TestNothingReachesARepositoryWithoutThePolicy(t *testing.T) {
	t.Parallel()

	files := auditedFiles(t)
	held := repositoryFields(files)
	if len(held) == 0 {
		t.Fatal("no type in this package holds a repository, so this test proved nothing")
	}

	audited := 0
	for _, source := range files {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			fields := held[receiverType(function)]
			if fields == nil {
				continue
			}
			reach := firstRepositoryCall(function, fields)
			if reach == token.NoPos {
				continue
			}
			audited++

			decided := firstCallTo(function.Body, "Authorize")
			if decided == token.NoPos {
				t.Errorf("%s: %s reaches the repository and never authorizes, so the Grant it passes was answered by nobody",
					source.path, function.Name.Name)
				continue
			}
			if decided > reach {
				t.Errorf("%s: %s reaches the repository before it authorizes",
					source.path, function.Name.Name)
			}
		}
	}
	if audited == 0 {
		t.Fatal("no method in this package reaches a repository, so this test proved nothing")
	}
}

// repositoryFields is, per type, the fields holding a repository.
func repositoryFields(files []parsedGoFile) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, source := range files {
		ast.Inspect(source.file, func(node ast.Node) bool {
			specification, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			structure, ok := specification.Type.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range structure.Fields.List {
				if !strings.HasSuffix(namedType(field.Type), "Repository") {
					continue
				}
				for _, name := range field.Names {
					if out[specification.Name.Name] == nil {
						out[specification.Name.Name] = map[string]bool{}
					}
					out[specification.Name.Name][name.Name] = true
				}
			}
			return true
		})
	}
	return out
}

// firstRepositoryCall is where a method first calls through one of its own
// repository fields.
func firstRepositoryCall(function *ast.FuncDecl, fields map[string]bool) token.Pos {
	receiver := receiverName(function)
	found := token.NoPos

	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		field, ok := method.X.(*ast.SelectorExpr)
		if !ok || !fields[field.Sel.Name] {
			return true
		}
		holder, ok := field.X.(*ast.Ident)
		if !ok || holder.Name != receiver {
			return true
		}
		if found == token.NoPos || call.Pos() < found {
			found = call.Pos()
		}
		return true
	})
	return found
}

// capabilities is what arandu.mod.toml declares, and what the code does, under
// the same four names.
type capabilities struct {
	network    bool
	filesystem bool
	exec       bool
	migrations bool
}

// TestTheDeclaredCapabilitiesAreWhatTheCodeDoes is the audit the installer
// cannot run. `aru doctor` compares an application's manifest against the
// application's code; the manifest of a package it installed is never opened,
// so this comparison exists here or nowhere.
//
// Both directions fail. Using more than was declared is the one that costs
// somebody something, and the doctor calls it an error too. Declaring more than
// is used is milder -- a warning, there -- and it fails here because `go test`
// has one outcome and because asking for what is not needed is how a
// declaration stops being worth reading.
func TestTheDeclaredCapabilitiesAreWhatTheCodeDoes(t *testing.T) {
	t.Parallel()

	declared := declaredCapabilities(t)
	used := usedCapabilities(auditedFiles(t))

	for _, capability := range []struct {
		name        string
		declared    bool
		used        bool
		consequence string
	}{
		{"network", declared.network, used.network,
			"the code makes calls that leave the process, and whoever installs it agreed to a package that does not"},
		{"filesystem", declared.filesystem, used.filesystem,
			"the code reads or writes files outside the database, which nothing in its API shows"},
		{"exec", declared.exec, used.exec,
			"the code runs another program, which is the widest capability there is"},
		{"migrations", declared.migrations, used.migrations,
			"the code owns tables, and an installer who did not expect that will not know to migrate before deploying"},
	} {
		switch {
		case capability.used && !capability.declared:
			t.Errorf("arandu.mod.toml declares %s = false and the code uses it: %s. Declare it, or remove what needs it",
				capability.name, capability.consequence)
		case capability.declared && !capability.used:
			t.Errorf("arandu.mod.toml declares %s = true and nothing uses it: set %s = false, or the declaration stops being worth reading",
				capability.name, capability.name)
		}
	}
}

// declaredCapabilities reads the [permissions] block.
//
// By hand, because the alternative is a third dependency in a module that is
// compiled into other people's builds, and four booleans do not earn one.
func declaredCapabilities(t *testing.T) capabilities {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(packageRoot(t), "arandu.mod.toml"))
	if err != nil {
		t.Fatalf("reading arandu.mod.toml: %v", err)
	}

	var declared capabilities
	seen := map[string]bool{}
	inside := false

	for _, line := range strings.Split(string(body), "\n") {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inside = line == "[permissions]"
			continue
		}
		if !inside {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		seen[key] = true
		switch key {
		case "network":
			declared.network = value == "true"
		case "filesystem":
			declared.filesystem = value == "true"
		case "exec":
			declared.exec = value == "true"
		case "migrations":
			declared.migrations = value == "true"
		}
	}

	for _, required := range []string{"network", "filesystem", "exec", "migrations"} {
		if !seen[required] {
			t.Errorf("arandu.mod.toml leaves %s undeclared under [permissions]; silence reads as false to whoever installs this, and a capability nobody declared is one nobody agreed to",
				required)
		}
	}
	return declared
}

// usedCapabilities is what the code actually does, by syntax.
//
// It reads calls rather than imports, for the reason the manifest gives: this
// package imports net/http for a method constant and a request type, and an
// import says nothing about whether anything leaves the process.
func usedCapabilities(files []parsedGoFile) capabilities {
	var used capabilities

	for _, source := range files {
		if strings.Contains(source.path, "migrations/") {
			used.migrations = true
		}
		for _, imported := range source.file.Imports {
			switch strings.Trim(imported.Path.Value, `"`) {
			case "os/exec":
				used.exec = true
			case "net/smtp", "net/rpc":
				used.network = true
			case "io/ioutil":
				used.filesystem = true
			}
		}

		standard := standardImports(source.file)
		ast.Inspect(source.file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.CallExpr:
				switch name := qualifiedName(node, standard); {
				case name == "http.Get", name == "http.Post", name == "http.Head",
					name == "http.PostForm", name == "http.NewRequest",
					name == "http.NewRequestWithContext",
					name == "net.Dial", name == "net.DialTimeout",
					strings.HasSuffix(name, ".Do"):
					used.network = true

				case name == "os.Open", name == "os.OpenFile", name == "os.Create",
					name == "os.ReadFile", name == "os.WriteFile", name == "os.Remove",
					name == "os.RemoveAll", name == "os.Mkdir", name == "os.MkdirAll",
					name == "os.Rename", name == "os.ReadDir",
					name == "filepath.Walk", name == "filepath.WalkDir":
					used.filesystem = true

				case name == "exec.Command", name == "exec.CommandContext", name == "syscall.Exec":
					used.exec = true
				}
			case *ast.FuncDecl:
				if node.Recv != nil && node.Name.Name == "Migrations" {
					used.migrations = true
				}
			}
			return true
		})
	}
	return used
}

// standardImports maps the local name of every standard library import to the
// package's own name, so an aliased import is recognised by what it is.
//
// Only the standard library, because the names matched above are its names. A
// third-party package whose last element is "http" is not net/http, and reading
// it as one reports a router helper as an outbound call.
func standardImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, imported := range file.Imports {
		path := strings.Trim(imported.Path.Value, `"`)
		if first, _, _ := strings.Cut(path, "/"); strings.Contains(first, ".") {
			continue
		}

		name := path
		if slash := strings.LastIndexByte(path, '/'); slash >= 0 {
			name = path[slash+1:]
		}
		local := name
		if imported.Name != nil {
			local = imported.Name.Name
		}
		out[local] = name
	}
	return out
}

// qualifiedName is a call as "package.Function" where the receiver is an
// imported package, and ".Method" where it is a value.
func qualifiedName(call *ast.CallExpr, standard map[string]string) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		if identifier, ok := call.Fun.(*ast.Ident); ok {
			return identifier.Name
		}
		return ""
	}
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return "." + selector.Sel.Name
	}
	if name, imported := standard[owner.Name]; imported {
		return name + "." + selector.Sel.Name
	}
	return owner.Name + "." + selector.Sel.Name
}
