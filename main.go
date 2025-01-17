package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "embed"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/ast/astutil"
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

//go:embed lexicue.cue
var lexicueSource string

var useMap = flag.Bool("m", false, "generate map entries rather than top level definitions")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: lexicue [lexiconfile.json | directory]...\n")
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
	}
	ctx := cuecontext.New()
	lexiconTypes := ctx.CompileString(lexiconSchemaSource, cue.Filename("lexicon.cue"))
	if err := lexiconTypes.Err(); err != nil {
		log.Fatalf("cannot compile lexicon schema: %v", err)
	}
	lexiconSchema := lexiconTypes.LookupPath(cue.MakePath(cue.Def("#LexiconDoc")))
	if err := lexiconSchema.Err(); err != nil {
		log.Fatal(err)
	}
	moduleRoot := "lexicon.me"
	if *useMap {
		moduleRoot += "/defs"
	}
	fmt.Printf("exec cue vet ./...\n")
	fmt.Println()
	fmt.Printf("-- cue.mod/module.cue --\n")
	fmt.Printf("module: %q\n", moduleRoot)
	fmt.Printf("language: version: %q\n", "v0.10.0")
	deps := &dependencies{
		arcs: make(map[arc]map[arc]bool),
	}
	for _, arg := range flag.Args() {
		if arg == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "cannot read <stdin>: %v\n", err)
				continue
			}
			if err := genCUEFromJSONData(data, "<stdin>", lexiconSchema, deps, moduleRoot); err != nil {
				fmt.Fprintf(os.Stderr, "<stdin>: %v\n", err)
				return
			}
			continue
		}
		for w := fs.Walk(arg); w.Step(); {
			if err := w.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", w.Path(), err)
				continue
			}
			if w.Stat().IsDir() {
				continue
			}
			if !strings.HasSuffix(w.Path(), ".json") {
				continue
			}
			if err := genCUE(w.Path(), lexiconSchema, deps, moduleRoot); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", w.Path(), err)
			}
		}
	}
	fmt.Printf("-- cue.mod/pkg/cueschemas.org/lexicue/lexicue.cue --\n")
	fmt.Printf("%s\n", lexicueSource)
	fmt.Printf("-- deps.mermaid --\n")
	printDeps(deps, moduleRoot)
	fmt.Printf("-- cycles --\n")
	printCycles(deps, moduleRoot)
}

type dependencies struct {
	arcs map[arc]map[arc]bool
}

type arc struct {
	from, to string
}

func printCycles(deps *dependencies, moduleRoot string) {
	arcs := make(map[string][]string)
	for d := range deps.arcs {
		arcs[d.from] = append(arcs[d.from], d.to)
	}
	cycles := make(map[string]bool)
	for d := range arcs {
		visit := make(map[string]bool)
		checkCycle(cycles, d, arcs, visit, nil)
	}
	if len(cycles) == 0 {
		return
	}
	for c := range cycles {
		pkgs := strings.Split(c, " -> ")
		for i, pkg := range pkgs {
			fmt.Printf("%s", strings.TrimPrefix(pkg, moduleRoot+"/"))
			if i < len(pkgs)-1 {
				fmt.Printf(" ->\n")
			} else {
				fmt.Println()
				break
			}
			identArcs := deps.arcs[arc{pkgs[i], pkgs[i+1]}]
			for ia := range identArcs {
				to := ia.to
				if to == "" {
					to = "*"
				}
				fmt.Printf("\t%s -> %s\n", ia.from, to)
			}
		}
		fmt.Println()
	}
}

func checkCycle(cycles map[string]bool, pkg string, arcs map[string][]string, visit map[string]bool, path []string) {
	if visit[pkg] {
		cp := append([]string(nil), path...)
		for i, p := range cp {
			if p == pkg {
				cp = cp[i:]
				break
			}
		}

		cp = leastFirst(cp)
		cp = append(cp, cp[0])
		cycles[strings.Join(cp, " -> ")] = true
		return
	}
	visit[pkg] = true
	path = append(path, pkg)
	for _, d := range arcs[pkg] {
		checkCycle(cycles, d, arcs, visit, path)
	}
	delete(visit, pkg)
}

func leastFirst(xs []string) []string {
	if len(xs) <= 1 {
		return xs
	}
	min := xs[0]
	minIndex := 0
	for i, s := range xs {
		if s < min {
			min, minIndex = s, i
		}
	}
	if minIndex == 0 {
		return xs
	}
	xs1 := make([]string, 0, len(xs))
	for i := range xs {
		i1 := (i + minIndex) % len(xs)
		xs1 = append(xs1, xs[i1])
	}
	return xs1
}

func printDeps(deps *dependencies, moduleRoot string) {
	fmt.Printf("flowchart LR\n")
	ids := make(map[string]string)
	nodeID := func(name string) string {
		if id, ok := ids[name]; ok {
			return id
		}
		id := fmt.Sprintf("id%d", len(ids))
		ids[name] = id
		fmt.Printf("\t%s[%s]\n", id, strings.TrimPrefix(name, moduleRoot+"/"))
		return id
	}
	for d := range deps.arcs {
		if d.to == "cueschemas.org/lexicue" {
			continue
		}
		fmt.Printf("\t%s --> %s\n", nodeID(d.from), nodeID(d.to))
	}
}

type generator struct {
	pkg          string
	currentDef   string
	id           string
	useMap       bool
	importsByPkg map[string]*ast.Ident
	deps         *dependencies
	moduleRoot   string
}

func genCUE(f string, lexiconSchema cue.Value, deps *dependencies, moduleRoot string) error {
	defer func() {
		if err := recover(); err != nil {
			panic(fmt.Errorf("panic on %q: %v", f, err))
		}
	}()
	data, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	return genCUEFromJSONData(data, f, lexiconSchema, deps, moduleRoot)
}

func genCUEFromJSONData(data []byte, filename string, lexiconSchema cue.Value, deps *dependencies, moduleRoot string) error {
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return err
	}
	if err := validateJSON(data, filename, lexiconSchema); err != nil {
		return fmt.Errorf("cue validate: %v", errors.Details(err, nil))
	}
	g := &generator{
		id:           schema.ID,
		useMap:       *useMap,
		importsByPkg: make(map[string]*ast.Ident),
		moduleRoot:   moduleRoot,
		deps:         deps,
	}
	if g.useMap {
		g.pkg = moduleRoot
	} else {
		pkg, err := g.id2Pkg(g.id)
		if err != nil {
			return err
		}
		g.pkg = pkg
	}
	astf := &ast.File{
		Decls: []ast.Decl{
			&ast.Package{
				Name: ast.NewIdent(impliedImportIdent(g.pkg)),
			},
		},
	}
	defs := &ast.StructLit{}

	// Process main first, so it appears at the top.
	for name, t := range schema.Defs {
		if name != "main" {
			continue
		}
		g.currentDef = "#main"
		e, err := g.cueForDefinition(t, name)
		if err != nil {
			return fmt.Errorf("bad schema for %q: %v", name, err)
		}
		if g.useMap {
			addField(defs, g.id+g.currentDef, regular, e, t.Description)
			continue
		}
		// Main description becomes package doc comment.
		setDescription(astf.Decls[0], t.Description)

		if _, ok := e.(*ast.StructLit); ok {
			// It's a definition, so close it up by defining it in _#def first,
			// and then embedding that.
			astf.Decls = append(astf.Decls,
				&ast.Field{
					Label: ast.NewIdent("_#def"),
					Value: e,
				},
				&ast.EmbedDecl{
					Expr: ast.NewIdent("_#def"),
				},
			)
		} else {
			astf.Decls = append(astf.Decls, &ast.EmbedDecl{
				Expr: e,
			})
		}
	}

	for _, name := range sortedKeys(schema.Defs) {
		if name == "main" {
			// Already processed.
			continue
		}
		t := schema.Defs[name]
		g.currentDef = "#" + name
		e, err := g.cueForType(t, true)
		if err != nil {
			return fmt.Errorf("bad schema for %q: %v", name, err)
		}
		if g.useMap {
			addField(defs, g.id+"#"+name, regular, e, t.Description)
		} else {
			astf.Decls = append(astf.Decls, &ast.Field{
				Label: ast.NewIdent(g.currentDef),
				Value: e,
			})
			setDescription(astf.Decls[len(astf.Decls)-1], t.Description)
		}
	}
	if g.useMap && len(defs.Elts) > 0 {
		astf.Decls = append(astf.Decls, &ast.Field{
			Label: ast.NewIdent("#def"),
			Value: defs,
		})
	}
	if err := astutil.Sanitize(astf); err != nil {
		return fmt.Errorf("cannot sanitize %q: %v", filename, err)
	}
	outData, err := format.Node(astf)
	if err != nil {
		return fmt.Errorf("cannot format source: %v (%v)", err, errors.Details(err, nil))
	}
	if g.useMap {
		fmt.Printf("-- %s/%s.cue --\n", strings.TrimPrefix(g.pkg, moduleRoot+"/"), g.id)
	} else {
		fmt.Printf("-- %s/defs.cue --\n", strings.TrimPrefix(g.pkg, moduleRoot+"/"))
	}
	os.Stdout.Write(outData)
	return nil
}

func (g *generator) cueForDefinition(t *TypeSchema, defName string) (ast.Expr, error) {
	switch t.Type {
	case "query":
		e := &ast.StructLit{}
		g.addXRPCBodyField(e, "output", t.Output)
		if t.Parameters != nil {
			parametersExpr, err := g.cueForType(t.Parameters, false)
			if err != nil {
				return nil, err
			}
			addField(e, "parameters", required, parametersExpr, t.Parameters.Description)
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
			addField(e, "key", regular, stringLit(t.Key), "")
		}
		record, err := g.cueForType(t.Record, false)
		if err != nil {
			return nil, err
		}
		addField(e, "record", required, record, t.Record.Description)
		return g.lexiconValue("record", e), nil
	case "subscription":
		e := &ast.StructLit{}
		params, err := g.cueForType(t.Parameters, false)
		if err != nil {
			return nil, err
		}
		addField(e, "parameters", required, params, t.Parameters.Description)
		if t.Message != nil {
			schema, err := g.cueForType(t.Message.Schema, false)
			if err != nil {
				return nil, err
			}
			addField(e, "message", required, &ast.StructLit{
				Elts: []ast.Decl{
					&ast.Field{
						Label: ast.NewIdent("schema"),
						Value: schema,
					},
				},
			}, t.Message.Schema.Description)
		}
		return g.lexiconValue("subscription", e), nil
	case "image":
		e := &ast.StructLit{}
		addMaxConstraint(e, t.MaxWidth, "width")
		addMaxConstraint(e, t.MaxHeight, "height")
		addMaxConstraint(e, t.MaxSize, "size")
		return g.lexiconValue("image", e), nil
	case "video":
		e := &ast.StructLit{}
		addMaxConstraint(e, t.MaxWidth, "width")
		addMaxConstraint(e, t.MaxHeight, "height")
		addMaxConstraint(e, t.MaxLength, "length")
		addMaxConstraint(e, t.MaxSize, "size")
		return g.lexiconValue("video", e), nil
	case "audio":
		e := &ast.StructLit{}
		addMaxConstraint(e, t.MaxLength, "length")
		addMaxConstraint(e, t.MaxSize, "size")
		return g.lexiconValue("audio", e), nil
	default:
		e, err := g.cueForType(t, true)
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
	addField(lit, fieldName, regular, e, "")
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
		schemaExpr, err := g.cueForType(body.Schema, false)
		if err != nil {
			return nil, err
		}
		addField(e, "schema", regular, schemaExpr, body.Schema.Description)
	}
	return e, nil
}

func (g *generator) cueForType(t *TypeSchema, topLevel bool) (ast.Expr, error) {
	switch t.Type {
	case "token":
		if !topLevel {
			return nil, fmt.Errorf("token not defined at top level")
		}
		n := g.id
		if g.currentDef != "#main" {
			n += g.currentDef
		}
		return g.lexiconValue("token", stringLit(n)), nil
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
			pt := t.Properties[name]
			e, err := g.cueForType(pt, false)
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
			setDescription(f, pt.Description)
			lit.Elts = append(lit.Elts, f)
		}
		return lit, nil
	case "blob":
		e := &ast.StructLit{}
		addMaxConstraint(e, t.MaxSize, "size")
		addMimeType(e, t.Accept)
		return g.lexiconValue("blob", e), nil
	case "cid-link":
		return g.lexiconValue("cidLink", nil), nil
	case "array":
		itemType, err := g.cueForType(t.Items, false)
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
				Fun: g.externalRef("list", "MinItems"),
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
				Fun: g.externalRef("list", "MaxItems"),
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
	def := g.externalRef("cueschemas.org/lexicue", kind)
	if of == nil {
		return def
	}
	if slit, ok := of.(*ast.StructLit); ok && len(slit.Elts) == 0 {
		return def
	}
	return &ast.BinaryExpr{
		X:  def,
		Op: token.AND,
		Y:  of,
	}
}

func (g *generator) refExpr(name string) (ast.Expr, error) {
	if g.useMap {
		if strings.HasPrefix(name, "#") {
			name = g.id + name
		}
		return &ast.IndexExpr{
			X:     ast.NewIdent("#def"),
			Index: stringLit(name),
		}, nil
	}
	path, def, ok := strings.Cut(name, "#")
	if ok && path == "" {
		// Local reference.
		return ast.NewIdent("#" + def), nil
	}
	pkg, err := g.id2Pkg(path)
	if err != nil {
		return nil, err
	}
	if pkg == g.pkg {
		return ast.NewIdent("#" + def), nil
	}
	ref := ""
	if ok {
		ref = "#" + def
	}
	return g.externalRef(pkg, ref), nil
}

func (g *generator) externalRef(pkg string, ident string) ast.Expr {
	a := arc{g.pkg, pkg}
	m := g.deps.arcs[a]
	if m == nil {
		m = make(map[arc]bool)
		g.deps.arcs[a] = m
	}
	m[arc{g.currentDef, ident}] = true
	if ident == "" {
		return g.addImport(pkg)
	}
	return &ast.SelectorExpr{
		X:   g.addImport(pkg),
		Sel: ast.NewIdent(ident),
	}
}

func (g *generator) addImport(pkg string) *ast.Ident {
	if ident := g.importsByPkg[pkg]; ident != nil {
		return ident
	}
	id := impliedImportIdent(pkg)
	ispec := &ast.ImportSpec{
		Path: stringLit(pkg),
	}
	if id == "defs" {
		// defs is commonly used and meaningless, so try
		// for a more informative identifier.
		id1 := path.Base(path.Dir(pkg))
		id1 = strings.ReplaceAll(id1, ".", "_")
		if id1 != "" && ast.IsValidIdent(id1) {
			ispec.Name = ast.NewIdent(id1)
			id = id1
		}
	}
	ident := &ast.Ident{
		Name: id,
		Node: ispec,
	}
	g.importsByPkg[pkg] = ident
	return ident
}

func addMimeType(e *ast.StructLit, accept []string) {
	if len(accept) == 0 {
		return
	}
	var v ast.Expr
	for _, s := range accept {
		var elt ast.Expr

		if strings.HasSuffix(s, "/*") {
			// TODO what's the general matching pattern syntax here?
			elt = &ast.UnaryExpr{
				Op: token.MAT,
				X:  stringLit("^" + regexp.QuoteMeta(strings.TrimSuffix(s, "*"))),
			}
		} else {
			elt = stringLit(s)
		}
		if v == nil {
			v = elt
		} else {
			v = or(v, elt)
		}
	}
	addField(e, "mimeType", required, v, "")
}

func addMaxConstraint(e *ast.StructLit, n *int, fieldName string) {
	if n == nil {
		return
	}
	addField(e, fieldName, required, &ast.UnaryExpr{
		Op: token.LEQ,
		X:  numericLit(*n, false),
	}, "")
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

func setDescription(n ast.Node, desc string) {
	if desc == "" {
		return
	}
	ast.SetComments(n, []*ast.CommentGroup{{
		Doc: true,
		List: []*ast.Comment{{
			Text: "// " + desc,
		}},
	}})
}

func inSlice[T comparable](x T, xs []T) bool {
	for _, y := range xs {
		if x == y {
			return true
		}
	}
	return false
}

func (g *generator) id2Pkg(p string) (string, error) {
	parts := strings.Split(p, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("not enough elements in path %q", p)
	}
	var buf strings.Builder
	buf.WriteString(g.moduleRoot + "/")
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

type constraint = token.Token

const (
	regular  = token.ILLEGAL
	optional = token.OPTION
	required = token.NOT
)

func addField(lit *ast.StructLit, fieldName string, kind constraint, e ast.Expr, description string) {
	var label ast.Label
	if ast.IsValidIdent(fieldName) {
		label = ast.NewIdent(fieldName)
	} else {
		label = stringLit(fieldName)
	}
	f := &ast.Field{
		Label:      label,
		Constraint: kind,
		Value:      e,
	}
	setDescription(f, description)
	lit.Elts = append(lit.Elts, f)
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

func source(n ast.Node) string {
	data, err := format.Node(n)
	if err != nil {
		panic(err)
	}
	return string(data)
}
