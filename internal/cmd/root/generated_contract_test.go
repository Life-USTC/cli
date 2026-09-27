package root

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Life-USTC/CLI/internal/output"
)

func TestSpecGeneratedBusinessClients(t *testing.T) {
	t.Run("openapi.cli-generated-business-client", func(t *testing.T) {
		assertGeneratedBusinessCalls(t)
		t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
		for _, invalid := range []bool{false, true} {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != "GET" || r.URL.Path != "/api/publications" || r.URL.Query().Get("pageSize") != "2" || r.URL.Query().Get("type") != "notice" {
					t.Errorf("unexpected generated request: %s %s", r.Method, r.URL)
				}
				w.Header().Set("Content-Type", "application/json")
				if invalid {
					_, _ = io.WriteString(w, `{"data":[],"pagination":{"page":"wrong-type"}}`)
					return
				}
				_, _ = io.WriteString(w, `{"data":[{"id":"p1","publicationType":"notice","revision":{"title":"Scholarship","summary":"Keep summary","observedAt":"2026-09-01T00:00:00Z"},"source":{"name":"Campus"}}],"pagination":{"page":1,"pageSize":2,"total":1,"totalPages":1}}`)
			}))
			text, err := executeGeneratedCommand(t, []string{"--server", server.URL, "--json", "catalog", "publication", "--type", "notice", "--limit", "2"})
			server.Close()
			if requests != 1 {
				t.Fatalf("requests = %d", requests)
			}
			if invalid {
				if err == nil || text != "" {
					t.Fatalf("invalid typed response must fail without output: %q, %v", text, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]any
				if err := json.Unmarshal([]byte(text), &body); err != nil {
					t.Fatal(err)
				}
				row := body["data"].([]any)[0].(map[string]any)
				if row["id"] != "p1" || row["revision"].(map[string]any)["summary"] != "Keep summary" {
					t.Fatalf("typed output lost fields: %s", text)
				}
			}
		}
	})
}

func executeGeneratedCommand(t *testing.T, args []string) (string, error) {
	t.Helper()
	oldOutput, oldStdout := output.Current, os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	done := make(chan string)
	go func() { data, _ := io.ReadAll(reader); done <- string(data) }()
	cmd := NewCmdRoot()
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(args)
	err = cmd.Execute()
	_ = writer.Close()
	os.Stdout, output.Current = oldStdout, oldOutput
	result := <-done
	_ = reader.Close()
	return result, err
}

// Inspect all production commands, and match the response type at each actual
// generated call to its generated operation, rather than merely checking imports.
func assertGeneratedBusinessCalls(t *testing.T) {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	fset := token.NewFileSet()
	generated, err := parser.ParseFile(fset, filepath.Join(root, "internal/openapi/client.gen.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
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
					t.Errorf("untyped business transport at %s", fset.Position(call.Pos()))
				}
				if receiver.Name == "http" && (method.Sel.Name == "NewRequest" || method.Sel.Name == "NewRequestWithContext" || method.Sel.Name == "Get" || method.Sel.Name == "Post") {
					t.Errorf("handwritten HTTP business request at %s", fset.Position(call.Pos()))
				}
			}
			if method.Sel.Name == "DoJSON" || method.Sel.Name == "DoRaw" {
				t.Errorf("raw business request at %s", fset.Position(call.Pos()))
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
				t.Errorf("generated %s bypasses typed response at %s", op, fset.Position(call.Pos()))
				return true
			}
			generic, ok := parent.Fun.(*ast.IndexExpr)
			if !ok {
				t.Errorf("generated %s requires a typed decoder", op)
				return true
			}
			decoder, ok := generic.X.(*ast.SelectorExpr)
			if !ok || (decoder.Sel.Name != "ParseResponse" && decoder.Sel.Name != "ReadTypedResponse") {
				t.Errorf("generated %s uses unknown decoder", op)
				return true
			}
			actual := strings.ReplaceAll(typeText(fset, generic.Index), "openapi.", "")
			actual = strings.ReplaceAll(actual, "any", "interface{}")
			if !allowed[actual] {
				t.Errorf("%s decodes %s; declared responses %v", op, actual, allowed)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls == 0 {
		t.Fatal("no generated business operations inspected")
	}
}

func typeText(fset *token.FileSet, expr ast.Expr) string {
	var out bytes.Buffer
	_ = format.Node(&out, fset, expr)
	return out.String()
}
