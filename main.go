package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	_ "embed"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/token"
	"github.com/kr/fs"
)

// TODO comments
// TODO imports/references

//go:embed lexicon.cue
var lexiconSchemaSource string

func main() {
	ctx := cuecontext.New()
	lexiconTypes := ctx.CompileString(lexiconSchemaSource, cue.Filename("lexicon.cue"))
	if err := lexiconTypes.Err(); err != nil {
		log.Fatalf("cannot compile lexicon schema: %v", err)
	}
	lexiconSchema := lexiconTypes.LookupPath(cue.MakePath(cue.Def("#LexiconDoc")))
	if err := lexiconSchema.Err(); err != nil {
		log.Fatal(err)
	}
	w := fs.Walk("/home/rogpeppe/other/bluesky/atproto/lexicons")
	n := 0
	fmt.Printf("-- cue.mod/module.cue --\n")
	fmt.Printf("module: \"test.org\"\n")
	deps := make(map[dep]bool)
	for w.Step() {
		if w.Stat().IsDir() {
			continue
		}
		if !strings.HasSuffix(w.Path(), ".json") {
			continue
		}
		if err := genCUE(w.Path(), lexiconSchema, deps); err != nil {
			fmt.Printf("%s: %v\n", w.Path(), err)
			return
		}
		n++
	}
	fmt.Printf("-- deps.mermaid --\n")
	printDeps(deps)
	fmt.Printf("-- cycles --\n")
	printCycles(deps)
	fmt.Printf("%d schemas checked\n", n)
}

func printCycles(deps map[dep]bool) {
	arcs := make(map[string][]string)
	for d := range deps {
		arcs[d.from] = append(arcs[d.from], d.to)
	}
	cycles := make(map[string]bool)
	for d := range arcs {
		visit := make(map[string]bool)
		checkCycle(cycles, d, arcs, visit, nil)
	}
	for c := range cycles {
		fmt.Printf("%s\n", c)
	}
}

func checkCycle(cycles map[string]bool, pkg string, arcs map[string][]string, visit map[string]bool, path []string) {
	if visit[pkg] {
		cp := path
		for i, p := range cp {
			if p == pkg {
				cp = cp[i:]
				break
			}
		}
		cycles[strings.Join(rev(append(cp, pkg)), " -> ")] = true
		return
	}
	visit[pkg] = true
	path = append(path, pkg)
	for _, d := range arcs[pkg] {
		checkCycle(cycles, d, arcs, visit, path)
	}
	delete(visit, pkg)
}

func printDeps(deps map[dep]bool) {
	fmt.Printf("flowchart LR\n")
	ids := make(map[string]string)
	nodeID := func(name string) string {
		if id, ok := ids[name]; ok {
			return id
		}
		id := fmt.Sprintf("id%d", len(ids))
		ids[name] = id
		fmt.Printf("\t%s[%s]\n", id, name)
		return id
	}
	for d := range deps {
		if d.to == "cueschemas.org/lexicue" {
			continue
		}
		fmt.Printf("\t%s --> %s\n", nodeID(d.from), nodeID(d.to))
	}
}

type dep struct {
	from, to string
}

type generator struct {
	pkg            string
	defs           map[string]*TypeSchema
	cueDefs        map[string]ast.Expr
	importsByPkg   map[string]string
	importsByIdent map[string]string
}

func genCUE(f string, lexiconSchema cue.Value, deps map[dep]bool) error {
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("panic on %q: %v", f, err))
		}
	}()
	data, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return err
	}
	if err := validateJSON(data, f, lexiconSchema); err != nil {
		return fmt.Errorf("cue validate: %v", errors.Details(err, nil))
	}
	pkg, err := id2Pkg(schema.ID)
	if err != nil {
		return err
	}
	g := &generator{
		pkg:            pkg,
		defs:           schema.Defs,
		cueDefs:        make(map[string]ast.Expr),
		importsByPkg:   make(map[string]string),
		importsByIdent: make(map[string]string),
	}
	astf := &ast.File{
		Decls: []ast.Decl{
			&ast.Package{
				Name: ast.NewIdent(impliedImportIdent(pkg)),
			},
		},
	}
	for _, name := range sortedKeys(schema.Defs) {
		e, err := g.cueForDefinition(schema.Defs[name])
		if err != nil {
			return fmt.Errorf("bad schema for %q: %v", name, err)
		}
		astf.Decls = append(astf.Decls, &ast.Field{
			Label: ast.NewIdent(name),
			Value: e,
		})
	}
	for pkgPath, ident := range g.importsByPkg {
		ispec := &ast.ImportSpec{
			Path: stringLit(pkgPath),
		}
		if impliedImportIdent(pkgPath) != ident {
			ispec.Name = ast.NewIdent(ident)
		}
		astf.Imports = append(astf.Imports, ispec)
		deps[dep{g.pkg, pkgPath}] = true
	}
	if len(astf.Imports) > 0 {
		decls := make([]ast.Decl, len(astf.Decls)+1)
		decls[0] = astf.Decls[0]
		decls[1] = &ast.ImportDecl{
			Specs: astf.Imports,
		}
		copy(decls[2:], astf.Decls[1:])
		astf.Decls = decls
	}
	outData, err := format.Node(astf)
	if err != nil {
		return fmt.Errorf("cannot format source: %v (%v)", err, errors.Details(err, nil))
	}
	fmt.Printf("-- %s/defs.cue --\n", strings.TrimPrefix(g.pkg, "test.org/"))
	os.Stdout.Write(outData)
	return nil
}

func (g *generator) cueForDefinition(t *TypeSchema) (ast.Expr, error) {
	switch t.Type {
	case "query":
		e := &ast.StructLit{}
		g.addXRPCBodyField(e, "output", t.Output)
		if t.Parameters != nil {
			parametersExpr, err := g.cueForType(t.Parameters)
			if err != nil {
				return nil, err
			}
			addField(e, "parameters", parametersExpr)
		}
		return g.lexiconValue("query", e), nil
	case "procedure":
		e := &ast.StructLit{}
		g.addXRPCBodyField(e, "input", t.Input)
		g.addXRPCBodyField(e, "output", t.Output)
		// TODO errors
		return g.lexiconValue("procedure", e), nil
	case "record":
		e := &ast.StructLit{}
		if t.Key != "" {
			addField(e, "key", stringLit(t.Key))
		}
		record, err := g.cueForType(t.Record)
		if err != nil {
			return nil, err
		}
		addField(e, "record", record)
		return g.lexiconValue("record", e), nil
	case "subscription":
		e := &ast.StructLit{}
		params, err := g.cueForType(t.Parameters)
		if err != nil {
			return nil, err
		}
		addField(e, "parameters", params)
		if t.Message != nil {
			schema, err := g.cueForType(t.Message.Schema)
			if err != nil {
				return nil, err
			}
			addField(e, "message", &ast.StructLit{
				Elts: []ast.Decl{
					&ast.Field{
						Label: ast.NewIdent("schema"),
						Value: schema,
					},
				},
			})
		}
		return g.lexiconValue("subscription", e), nil
	case "image":
		return g.lexiconValue("image", ast.NewIdent("type_TODO")), nil
	case "video":
		return g.lexiconValue("video", ast.NewIdent("type_TODO")), nil
	case "audio":
		return g.lexiconValue("audio", ast.NewIdent("type_TODO")), nil
	default:
		e, err := g.cueForType(t)
		if err != nil {
			return nil, err
		}
		return e, nil
	}
}

func (g *generator) addXRPCBodyField(lit *ast.StructLit, fieldName string, body *BodyType) error {
	if body == nil {
		return nil
	}
	e, err := g.cueForXRPCBody(body)
	if err != nil {
		return err
	}
	addField(lit, fieldName, e)
	return nil
}

func (g *generator) cueForXRPCBody(body *BodyType) (ast.Expr, error) {
	e := &ast.StructLit{
		Elts: []ast.Decl{
			&ast.Field{
				Label: ast.NewIdent("encoding"),
				Value: stringLit(body.Encoding),
			},
		},
	}
	if body.Schema != nil {
		schemaExpr, err := g.cueForType(body.Schema)
		if err != nil {
			return nil, err
		}
		addField(e, "schema", schemaExpr)
	}
	return e, nil
}

func (g *generator) cueForType(t *TypeSchema) (ast.Expr, error) {
	switch t.Type {
	case "ref":
		return g.refExpr(t.Ref)
	case "union":
		if len(t.Refs) == 0 {
			return nil, fmt.Errorf("no elements in union")
		}
		var e ast.Expr
		for _, r := range t.Refs {
			e1, err := g.refExpr(r)
			if err != nil {
				return nil, err
			}
			if e == nil {
				e = e1
				continue
			}
			e = or(e, e1)
		}
		return e, nil
	case "token":
		return g.lexiconValue("token", &ast.StructLit{
			Elts: []ast.Decl{
				&ast.Field{
					Label: ast.NewIdent("token"),
					Value: ast.NewString(g.pkg + ".TODO"),
				},
			},
		}), nil
	case "object", "params":
		// TODO what's the difference between params and object?
		lit := &ast.StructLit{}
		required := make(map[string]bool)
		for _, field := range t.Required {
			required[field] = true
		}
		nullable := make(map[string]bool)
		for _, field := range t.Nullable {
			nullable[field] = true
		}
		for _, name := range sortedKeys(t.Properties) {
			e, err := g.cueForType(t.Properties[name])
			if err != nil {
				return nil, err
			}
			if nullable[name] {
				e = &ast.BinaryExpr{
					X:  e,
					Op: token.OR,
					Y:  ast.NewIdent("null"),
				}
			}
			f := &ast.Field{
				Label: ast.NewIdent(name),
				Value: e,
			}
			if required[name] {
				f.Constraint = token.NOT
			} else {
				f.Constraint = token.OPTION
			}
			lit.Elts = append(lit.Elts, f)
		}
		return lit, nil
	case "blob":
		// see /home/rogpeppe/other/indigo/lex/util/lex_types.go:141
		return ast.NewIdent("Blob_TODO"), nil
	case "cid-link":
		return ast.NewIdent("CIDLink_TODO"), nil
	case "array":
		itemType, err := g.cueForType(t.Items)
		if err != nil {
			return nil, err
		}
		var e ast.Expr = &ast.ListLit{
			Elts: []ast.Expr{
				&ast.Ellipsis{
					Type: itemType,
				},
			},
		}
		if t.MinLength != nil {
			e = and(e, &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent(g.addImport("list")),
					Sel: ast.NewIdent("MinItems"),
				},
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.INT,
						Value: fmt.Sprint(*t.MinLength),
					},
				},
			})
		}
		if t.MaxLength != nil {
			e = and(e, &ast.CallExpr{
				Fun: &ast.SelectorExpr{
					X:   ast.NewIdent(g.addImport("list")),
					Sel: ast.NewIdent("MaxItems"),
				},
				Args: []ast.Expr{
					&ast.BasicLit{
						Kind:  token.INT,
						Value: fmt.Sprint(*t.MaxLength),
					},
				},
			})
		}
		return e, nil
	case "boolean":
		if constVal, ok := t.Const.(bool); ok {
			return ast.NewIdent(fmt.Sprint(constVal)), nil
		}
		var e ast.Expr = ast.NewIdent("bool")
		if defaultVal, ok := t.Default.(bool); ok {
			e = withDefault(e, ast.NewIdent(fmt.Sprint(defaultVal)))
		}
		return e, nil
	case "number", "integer":
		isInt := t.Type == "integer"
		if t.Const != nil {
			return numericLit(t.Const, isInt), nil
		}
		var e ast.Expr
		if t.Enum != nil {
			if len(t.Enum) == 0 {
				return nil, fmt.Errorf("empty enum")
			}
			e = numericLit(t.Enum[0], isInt)
			for _, v := range t.Enum[1:] {
				e = or(e, numericLit(v, isInt))
			}
		} else {
			if isInt {
				e = ast.NewIdent("int")
			} else {
				e = ast.NewIdent("number")
			}
		}
		if t.Minimum != nil {
			e = and(e, &ast.UnaryExpr{
				Op: token.GEQ,
				X:  numericLit(t.Minimum, isInt),
			})
		}
		if t.Maximum != nil {
			e = and(e, &ast.UnaryExpr{
				Op: token.LEQ,
				X:  numericLit(t.Maximum, isInt),
			})
		}
		if t.Default != nil {
			e = withDefault(e, numericLit(t.Default, isInt))
		}
		return e, nil
	case "string":
		// TODO MaxGraphemes
		// TODO KnownValues ("foo" | "bar" | string ?)
		// TODO Format
		if t.Const != nil {
			return stringLit(t.Const.(string)), nil
		}
		var e ast.Expr
		if t.Enum != nil {
			if len(t.Enum) == 0 {
				return nil, fmt.Errorf("empty enum")
			}
			e = stringLit(t.Enum[0].(string))
			for _, v := range t.Enum[1:] {
				e = or(e, stringLit(v.(string)))
			}
		} else {
			e = ast.NewIdent("string")
		}
		// TODO MinLength
		// TODO MaxLength	(length in runes? bytes?)
		if t.Default != nil {
			e = withDefault(e, stringLit(t.Default.(string)))
		}
		return e, nil
	case "bytes":
		// TODO MaxLength
		return ast.NewIdent("bytes"), nil
	case "unknown":
		return ast.NewIdent("_"), nil
	default:
		return nil, fmt.Errorf("unknown type %q", t.Type)
	}
}

func (g *generator) lexiconValue(kind string, of ast.Expr) ast.Expr {
	ident := g.addImport("cueschemas.org/lexicue")
	return &ast.BinaryExpr{
		X: &ast.SelectorExpr{
			X:   ast.NewIdent(ident),
			Sel: ast.NewIdent(kind),
		},
		Op: token.AND,
		Y:  of,
	}
}

func (g *generator) refExpr(name string) (ast.Expr, error) {
	path, def, ok := strings.Cut(name, "#")
	if ok && path == "" {
		// Local reference.
		return ast.NewIdent(def), nil
	}
	pkg, err := id2Pkg(path)
	if err != nil {
		return nil, err
	}
	if pkg == g.pkg {
		return ast.NewIdent(def), nil
	}
	importIdent := g.addImport(pkg)
	if !ok {
		return ast.NewIdent(importIdent), nil
	}
	return &ast.SelectorExpr{
		X:   ast.NewIdent(importIdent),
		Sel: ast.NewIdent(def),
	}, nil
}

func (g *generator) addImport(pkg string) string {
	if ident := g.importsByPkg[pkg]; ident != "" {
		return ident
	}
	ident := impliedImportIdent(pkg)
	identRoot := ident
	for i := 0; i < 20; i++ {
		ident := identRoot
		if i > 0 {
			ident = fmt.Sprintf("%s_%d", ident, i)
		}
		if g.importsByIdent[ident] == "" {
			g.importsByIdent[ident] = pkg
			g.importsByPkg[pkg] = ident
			return ident
		}
	}
	panic(fmt.Errorf("too many packages named %q", pkg))
}

func impliedImportIdent(pkgPath string) string {
	return path.Base(pkgPath)
}

func withDefault(e ast.Expr, defaultVal ast.Expr) ast.Expr {
	return or(
		&ast.UnaryExpr{
			Op: token.MUL,
			X:  defaultVal,
		},
		e,
	)
}

func numericLit(val any, isInt bool) ast.Expr {
	if isInt {
		return &ast.BasicLit{
			Kind:  token.INT,
			Value: fmt.Sprint(val),
		}
	}
	return &ast.BasicLit{
		Kind: token.FLOAT,
		// TODO is this right?
		Value: fmt.Sprint(val),
	}
}

func stringLit(s string) *ast.BasicLit {
	// TODO choose appropriate kind of string literal depending on content.
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: strconv.Quote(s),
	}
}

func inSlice[T comparable](x T, xs []T) bool {
	for _, y := range xs {
		if x == y {
			return true
		}
	}
	return false
}

func id2Pkg(p string) (string, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("not enough elements in path %q", p)
	}
	var buf strings.Builder
	buf.WriteString("test.org/")
	for i := len(parts) - 2; i >= 0; i-- {
		if i < len(parts)-2 {
			buf.WriteByte('.')
		}
		buf.WriteString(parts[i])
	}
	fmt.Fprintf(&buf, "/%s", parts[len(parts)-1])
	return buf.String(), nil
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func and(x, y ast.Expr) ast.Expr {
	return &ast.BinaryExpr{
		X:  x,
		Op: token.AND,
		Y:  y,
	}
}

func or(x, y ast.Expr) ast.Expr {
	return &ast.BinaryExpr{
		X:  x,
		Op: token.OR,
		Y:  y,
	}
}

func addField(lit *ast.StructLit, fieldName string, e ast.Expr) {
	lit.Elts = append(lit.Elts, &ast.Field{
		Label: ast.NewIdent(fieldName),
		Value: e,
	})
}

// We'd use cuelang.org/go/encoding/json except for https://github.com/cue-lang/cue/issues/2395
func validateJSON(data []byte, filename string, schema cue.Value) error {
	v := schema.Context().CompileBytes(data, cue.Filename(filename))
	v = v.Unify(schema)
	return v.Validate(cue.Concrete(true))
}

func rev[T any](xs []T) []T {
	r := make([]T, 0, len(xs))
	for i := len(xs) - 1; i >= 0; i-- {
		r = append(r, xs[i])
	}
	return r
}
