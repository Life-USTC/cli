package root

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// Inspect all production commands, and match the response type at each actual
// generated call to its generated operation, rather than merely checking imports.
func auditGeneratedBusinessCalls() (int, []string) {
	issues := []string{}
	root := filepath.Join("..", "..", "..")
	fset := token.NewFileSet()
	generated, err := parser.ParseFile(fset, filepath.Join(root, "internal/openapi/client.gen.go"), nil, 0)
	if err != nil {
		return 0, []string{err.Error()}
	}
	models := map[string]map[string]bool{}
	ast.Inspect(generated, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || !strings.HasSuffix(spec.Name.Name, "Response") {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		op := strings.TrimSuffix(spec.Name.Name, "Response")
		models[op] = map[string]bool{}
		for _, field := range st.Fields.List {
			if len(field.Names) == 1 && strings.HasPrefix(field.Names[0].Name, "JSON2") {
				if ptr, ok := field.Type.(*ast.StarExpr); ok {
					models[op][typeText(fset, ptr.X)] = true
				}
			}
		}
		return true
	})
	calls := 0
	err = filepath.WalkDir(filepath.Join(root, "internal/cmd"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(filepath.ToSlash(path), "/apicmd/") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		// Every generated operation call is directly consumed by the typed decoder.
		parents := map[ast.Node]ast.Node{}
		var stack []ast.Node
		ast.Inspect(file, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				return false
			}
			if len(stack) > 0 {
				parents[node] = stack[len(stack)-1]
			}
			stack = append(stack, node)
			return true
		})
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if receiver, ok := method.X.(*ast.Ident); ok {
				if receiver.Name == "api" && (method.Sel.Name == "NewClient" || method.Sel.Name == "NewClientWithRefresh" || method.Sel.Name == "ReadResponse" || method.Sel.Name == "DecodeResponseBody") {
					issues = append(issues, fmt.Sprintf("untyped business transport at %s", fset.Position(call.Pos())))
				}
				if receiver.Name == "http" && (method.Sel.Name == "NewRequest" || method.Sel.Name == "NewRequestWithContext" || method.Sel.Name == "Get" || method.Sel.Name == "Post") {
					issues = append(issues, fmt.Sprintf("handwritten HTTP business request at %s", fset.Position(call.Pos())))
				}
			}
			if method.Sel.Name == "DoJSON" || method.Sel.Name == "DoRaw" {
				issues = append(issues, fmt.Sprintf("raw business request at %s", fset.Position(call.Pos())))
			}
			op := method.Sel.Name
			for _, suffix := range []string{"WithFormdataBody", "WithBody"} {
				op = strings.TrimSuffix(op, suffix)
			}
			allowed, generatedCall := models[op]
			if !generatedCall {
				return true
			}
			calls++
			// Binary downloads have no generated JSON success model.
			if len(allowed) == 0 {
				return true
			}
			parent, ok := parents[call].(*ast.CallExpr)
			if !ok {
				issues = append(issues, fmt.Sprintf("generated %s bypasses typed response at %s", op, fset.Position(call.Pos())))
				return true
			}
			generic, ok := parent.Fun.(*ast.IndexExpr)
			if !ok {
				issues = append(issues, fmt.Sprintf("generated %s requires a typed decoder", op))
				return true
			}
			decoder, ok := generic.X.(*ast.SelectorExpr)
			if !ok || (decoder.Sel.Name != "ParseResponse" && decoder.Sel.Name != "ReadTypedResponse") {
				issues = append(issues, fmt.Sprintf("generated %s uses unknown decoder", op))
				return true
			}
			actual := strings.ReplaceAll(typeText(fset, generic.Index), "openapi.", "")
			actual = strings.ReplaceAll(actual, "any", "interface{}")
			if !allowed[actual] {
				issues = append(issues, fmt.Sprintf("%s decodes %s; declared responses %v", op, actual, allowed))
			}
			return true
		})
		return nil
	})
	if err != nil {
		return 0, []string{err.Error()}
	}
	if calls == 0 {
		issues = append(issues, "no generated business operations inspected")
	}
	return calls, issues
}

func typeText(fset *token.FileSet, expr ast.Expr) string {
	var out bytes.Buffer
	_ = format.Node(&out, fset, expr)
	return out.String()
}
