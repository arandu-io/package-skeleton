package unit_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestTheApplicationIntegrationHasOneCompositionFile keeps the package's
// installation surface as explicit as its constructor. A package is composed
// in bootstrap/app.go; another bootstrap file would be another place a reader
// has to search to learn what the application contains.
func TestTheApplicationIntegrationHasOneCompositionFile(t *testing.T) {
	t.Parallel()

	root := packageRoot(t)
	guides := []string{
		"README.md",
		".agents/skills/skeleton-package/SKILL.md",
	}
	bootstrapFile := regexp.MustCompile(`bootstrap/[A-Za-z0-9_.-]+\.go`)

	for _, guide := range guides {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(guide)))
		if err != nil {
			t.Fatalf("reading %s: %v", guide, err)
		}

		text := string(body)
		for _, required := range []string{"bootstrap/app.go", ":module_slug.New(", "k.Register"} {
			if !strings.Contains(text, required) {
				t.Errorf("%s no longer teaches the explicit composition step %q", guide, required)
			}
		}

		for _, referenced := range bootstrapFile.FindAllString(text, -1) {
			if referenced != "bootstrap/app.go" {
				t.Errorf("%s introduces a second composition file %s", guide, referenced)
			}
		}
	}
}

// TestThePackageHasOneOwnerPerResponsibility freezes the application shapes
// this package is allowed to introduce. Repository and Model stay untouched:
// this guard covers composition and orchestration, not the data-path migration.
func TestThePackageHasOneOwnerPerResponsibility(t *testing.T) {
	t.Parallel()

	root := packageRoot(t)
	forbiddenPaths := []string{
		"app/Actions",
		"app/Data",
		"app/Rules",
		"app/Support",
		"bootstrap/providers.go",
		"routes/api.go",
		"routes/channels.go",
		"public/assets/manifest",
		"public/assets/manifest.json",
	}

	for _, path := range forbiddenPaths {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
			t.Errorf("refused application path %s exists in the package template", path)
		} else if !os.IsNotExist(err) {
			t.Errorf("checking refused application path %s: %v", path, err)
		}
	}

	inspectExplicitComposition(t, root)
	inspectServiceBoundary(t, root)
}

// inspectExplicitComposition rejects the names that would make registration
// implicit. AppServiceProvider is the deliberate nominal exception: when it is
// present it remains a Module, with no Register lifecycle of its own.
func inspectExplicitComposition(t *testing.T, root string) {
	t.Helper()

	forbiddenFunctions := map[string]bool{
		"DiscoverProviders": true,
		"Providers":         true,
		"RegisterProvider":  true,
		"RegisterProviders": true,
		"ResolveProvider":   true,
		"ResolveProviders":  true,
	}
	appProviderFound := false
	appProviderMethods := map[string]bool{}

	for _, source := range productionGoFiles(t, root) {
		compositionAliases := map[string]bool{}
		for _, imported := range source.file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Errorf("%s has an unreadable import %s", source.path, imported.Path.Value)
				continue
			}
			if strings.Contains(path, "/container/") || strings.HasSuffix(path, "/container") {
				t.Errorf("%s imports the container surface %s", source.path, path)
			}
			defaultName := ""
			switch path {
			case "github.com/arandu-io/framework/foundation", "github.com/arandu-io/hesape/foundation":
				defaultName = "foundation"
			case "github.com/arandu-io/framework/kernel":
				defaultName = "kernel"
			}
			if defaultName != "" {
				name := defaultName
				if imported.Name != nil {
					name = imported.Name.Name
				}
				if name == "." || name == "_" {
					t.Errorf("%s hides composition behind the import alias %q", source.path, name)
					continue
				}
				compositionAliases[name] = true
			}
		}

		ast.Inspect(source.file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.TypeSpec:
				if node.Name.Name != "Provider" && !strings.HasSuffix(node.Name.Name, "ServiceProvider") {
					return true
				}
				if node.Name.Name == "AppServiceProvider" {
					appProviderFound = true
					return true
				}
				t.Errorf("%s declares %s, a second composition abstraction", source.path, node.Name.Name)
			case *ast.FuncDecl:
				if node.Recv == nil && forbiddenFunctions[node.Name.Name] {
					t.Errorf("%s declares %s, which makes module registration implicit", source.path, node.Name.Name)
				}
				if receiverType(node) == "AppServiceProvider" {
					appProviderMethods[node.Name.Name] = true
				}
			case *ast.SelectorExpr:
				name, ok := node.X.(*ast.Ident)
				if ok && compositionAliases[name.Name] && node.Sel.Name == "Provider" {
					t.Errorf("%s uses a Provider contract instead of Module", source.path)
				}
			}
			return true
		})
	}

	if !appProviderFound {
		return
	}
	for _, method := range []string{"Name", "Routes"} {
		if !appProviderMethods[method] {
			t.Errorf("AppServiceProvider exists without the Module method %s", method)
		}
	}
	if appProviderMethods["Register"] {
		t.Error("AppServiceProvider declares a container-style Register lifecycle")
	}
}

// inspectServiceBoundary reads the route table rather than fixing a list of
// handler names in the test. Every handler registered there must call a Service
// and may not name a Repository directly.
func inspectServiceBoundary(t *testing.T, root string) {
	t.Helper()

	path := filepath.Join(root, "module.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parsing module.go: %v", err)
	}

	serviceFields := map[string]bool{}
	methods := map[string]*ast.FuncDecl{}
	var routes *ast.FuncDecl

	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range declaration.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != "Module" {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					t.Fatal("Module is not a struct")
				}
				for _, field := range structure.Fields.List {
					fieldType := namedType(field.Type)
					if strings.HasSuffix(fieldType, "Repository") {
						t.Errorf("Module holds %s directly; handlers must reach data through a Service", fieldType)
					}
					if !strings.HasSuffix(fieldType, "Service") {
						continue
					}
					for _, name := range field.Names {
						serviceFields[name.Name] = true
					}
				}
			}
		case *ast.FuncDecl:
			if receiverType(declaration) != "Module" {
				continue
			}
			methods[declaration.Name.Name] = declaration
			if declaration.Name.Name == "Routes" {
				routes = declaration
			}
		}
	}

	if len(serviceFields) == 0 {
		t.Fatal("Module has no Service collaborator for use-case orchestration")
	}
	if routes == nil {
		t.Fatal("Module has no Routes method")
	}

	handlers := registeredHandlers(routes)
	if len(handlers) == 0 {
		t.Fatal("Routes registers no Module handler")
	}
	for name := range handlers {
		handler := methods[name]
		if handler == nil {
			t.Errorf("Routes registers Module.%s, but no such method exists", name)
			continue
		}

		usesService := false
		receiver := receiverName(handler)
		ast.Inspect(handler.Body, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				field, ok := node.X.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				owner, ownedByModule := field.X.(*ast.Ident)
				if ownedByModule && owner.Name == receiver && serviceFields[field.Sel.Name] {
					usesService = true
				}
			case *ast.Ident:
				if strings.HasSuffix(node.Name, "Repository") || strings.EqualFold(node.Name, "repository") ||
					strings.EqualFold(node.Name, "repo") {
					t.Errorf("Module.%s reaches %s directly; use the Service boundary", name, node.Name)
				}
			}
			return true
		})
		if !usesService {
			t.Errorf("Module.%s does not delegate its use case to a Service", name)
		}
	}
}

type parsedGoFile struct {
	path string
	file *ast.File
}

func productionGoFiles(t *testing.T, root string) []parsedGoFile {
	t.Helper()

	files := []parsedGoFile{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") || strings.HasSuffix(entry.Name(), ".kyse.go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, parsedGoFile{path: filepath.ToSlash(relative), file: file})
		return nil
	})
	if err != nil {
		t.Fatalf("reading production Go files: %v", err)
	}
	return files
}

func registeredHandlers(routes *ast.FuncDecl) map[string]bool {
	handlers := map[string]bool{}
	receiver := receiverName(routes)
	ast.Inspect(routes.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		handler, ok := call.Args[len(call.Args)-1].(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, ok := handler.X.(*ast.Ident)
		if ok && owner.Name == receiver {
			handlers[handler.Sel.Name] = true
		}
		return true
	})
	return handlers
}

func receiverName(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 || len(function.Recv.List[0].Names) == 0 {
		return ""
	}
	return function.Recv.List[0].Names[0].Name
}

func receiverType(function *ast.FuncDecl) string {
	if function.Recv == nil || len(function.Recv.List) == 0 {
		return ""
	}
	return namedType(function.Recv.List[0].Type)
}

func namedType(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return namedType(expression.X)
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.IndexExpr:
		return namedType(expression.X)
	case *ast.IndexListExpr:
		return namedType(expression.X)
	default:
		return ""
	}
}

func packageRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("finding package root")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
