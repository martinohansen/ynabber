package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// strSlice is a helper flag.Value that collects repeated string flags into a
// slice.
type strSlice []string

func (s *strSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *strSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

// This tool parses the config.go file in the repository root and generates
// Markdown documentation for each exported struct (Config, Nordigen, YNAB).
//
// Usage (via go:generate):
//
//     go run ./cmd/gendocs > CONFIGURATION.md
//
// It can also be executed manually from the project root.

func main() {
	var (
		filePatterns strSlice
		title        string
		outPath      string
	)
	flag.Var(&filePatterns, "file", "path or glob pattern for source files")
	flag.StringVar(&title, "title", "Configuration", "title for markdown output")
	flag.StringVar(&outPath, "o", "", "write output to file (optional)")
	flag.Parse()

	var files []string
	for _, pattern := range filePatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fatal(err)
		}
		files = append(files, matches...)
	}

	if len(files) == 0 {
		fatal(fmt.Errorf("no files matched given -file patterns"))
	}

	fset := token.NewFileSet()

	// Collect exported struct type specs from all matched files.
	typeSpecs := map[string]*ast.TypeSpec{}

	for _, path := range files {
		absPath, err := filepath.Abs(path)
		if err != nil {
			fatal(err)
		}

		fileAst, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
		if err != nil {
			fatal(err)
		}

		for _, decl := range fileAst.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				// We only care about exported structs defined in this file.
				if !typeSpec.Name.IsExported() {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					// If the struct name already exists we prefer the first occurrence.
					if _, exists := typeSpecs[typeSpec.Name.Name]; !exists {
						typeSpecs[typeSpec.Name.Name] = typeSpec
					}
				}
			}
		}
	}

	// Deterministic order of sections.
	names := make([]string, 0, len(typeSpecs))
	for n := range typeSpecs {
		names = append(names, n)
	}
	sort.Strings(names)

	var out bytes.Buffer

	// Title
	fmt.Fprintf(&out, "# %s\n\n", title)
	fmt.Fprintf(&out, "This document is generated from configuration structs in the source code using `go generate`. **Do not edit manually.**\n\n")

	// Generate a section for each struct.
	for _, name := range names {
		spec := typeSpecs[name]
		structType := spec.Type.(*ast.StructType)

		// Section heading
		fmt.Fprintf(&out, "## %s\n\n", name)

		// Doc comment for type if present.
		if spec.Doc != nil {
			for _, c := range spec.Doc.List {
				trimmed := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
				trimmed = strings.Trim(trimmed, "/*")
				if trimmed != "" {
					fmt.Fprintln(&out, trimmed)
				}
			}
			fmt.Fprintln(&out)
		}

		// Markdown table header (Field column removed)
		fmt.Fprintln(&out, "| Environment variable | Type | Default | Description |")
		fmt.Fprintln(&out, "|:---------------------|:-----|:--------|:------------|")

		for _, field := range structType.Fields.List {
			// Skip embedded fields (Names == nil)
			if len(field.Names) == 0 {
				continue
			}

			// Field type as string
			typeStr := exprString(fset, field.Type)

			envTag, defTag := parseTags(field.Tag)
			// Skip fields without env var tag
			if envTag == "-" {
				continue
			}

			// Description – gather from Doc or Comment associated to field.
			desc := extractDoc(field)
			// Escape vertical bar used for markdown tables.
			desc = strings.ReplaceAll(desc, "|", "\\|")

			fmt.Fprintf(&out, "| %s | `%s` | %s | %s |\n", envTag, typeStr, defTag, desc)
		}

		fmt.Fprintln(&out)
	}

	if outPath == "" {
		// Write to stdout
		_, _ = os.Stdout.Write(out.Bytes())
		return
	}

	if err := os.WriteFile(outPath, out.Bytes(), 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "gendocs:", err)
	os.Exit(1)
}

// exprString converts an ast.Expr back to its source representation.
func exprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return buf.String()
}

// parseTags reads envconfig and default from a struct field tag.
func parseTags(tagLit *ast.BasicLit) (envVar, def string) {
	if tagLit == nil {
		return "-", "-"
	}
	tagValue, err := strconv.Unquote(tagLit.Value)
	if err != nil {
		return "-", "-"
	}
	tag := reflect.StructTag(tagValue)
	envVar = tag.Get("envconfig")
	def = tag.Get("default")

	if envVar == "" {
		envVar = "-"
	}
	if def == "" {
		def = "-"
	} else if !strings.Contains(def, "\n") && !strings.Contains(def, "`") {
		// wrap single-line value in backticks for readability
		def = fmt.Sprintf("`%s`", def)
	}
	return envVar, def
}

// extractDoc merges Doc and Comment groups for a struct field.
func extractDoc(field *ast.Field) string {
	var parts []string

	collect := func(cg *ast.CommentGroup) {
		if cg == nil {
			return
		}
		for _, c := range cg.List {
			txt := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			// Remove leading and trailing slash characters that might appear when using block comments,
			// but keep other symbols (e.g. "*" used for markdown lists).
			txt = strings.Trim(txt, "/")
			// Keep blank lines to preserve intended spacing inside markdown table.
			parts = append(parts, txt)
		}
	}

	collect(field.Doc)
	collect(field.Comment)

	// Join with <br> to render line breaks inside markdown table cells.
	return strings.Join(parts, "<br>")
}
