package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	cuejson "cuelang.org/go/encoding/json"
	"github.com/kr/fs"
)

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
	for w.Step() {
		if w.Stat().IsDir() {
			continue
		}
		if !strings.HasSuffix(w.Path(), ".json") {
			continue
		}
		if err := genCUE(w.Path(), lexiconSchema); err != nil {
			fmt.Printf("%s: %v\n", w.Path(), err)
			return
		}
		n++
	}
	fmt.Printf("%d schemas checked\n", n)
}

func genCUE(f string, lexiconSchema cue.Value) error {
	data, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return err
	}
	if err := cuejson.Validate(data, lexiconSchema); err != nil {
		return fmt.Errorf("cue validate: %v", errors.Details(err, nil))
	}
	g := &generator{
		id:              schema.ID,
		defs:            schema.Defs,
		lexiconPkgIdent: ast.NewIdent("lex"), // TODO import
		cueDefs:         make(map[string]ast.Expr),
	}
	astf := &ast.File{}
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
	outData, err := format.Node(astf)
	if err != nil {
		return fmt.Errorf("cannot format source: %v", err)
	}
	fmt.Printf("-- %s --\n", f)
	os.Stdout.Write(outData)
	return nil
}

func (g *generator) cueForDefinition(t *TypeSchema) (ast.Expr, error) {
	switch t.Type {
	case "query":
		return g.lexiconValue("query", ast.NewIdent("TODO")), nil
	case "procedure":
		return g.lexiconValue("procedure", ast.NewIdent("TODO")), nil
	case "record":
		return g.lexiconValue("record", ast.NewIdent("TODO")), nil
	case "image":
		return g.lexiconValue("image", ast.NewIdent("TODO")), nil
	case "video":
		return g.lexiconValue("video", ast.NewIdent("TODO")), nil
	case "audio":
		return g.lexiconValue("audio", ast.NewIdent("TODO")), nil
	case "subscription":
		return g.lexiconValue("subscription", ast.NewIdent("TODO")), nil
	default:
		e, err := g.cueForType(t)
		if err != nil {
			return nil, err
		}
		return e, nil
	}
}

type generator struct {
	id              string
	defs            map[string]*TypeSchema
	cueDefs         map[string]ast.Expr
	lexiconPkgIdent ast.Expr
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
					Value: ast.NewString(g.id + ".TODO"),
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
					X:   ast.NewIdent("list"),
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
					X:   ast.NewIdent("list"),
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

func stringLit(s string) ast.Expr {
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

func (g *generator) lexiconValue(kind string, of ast.Expr) ast.Expr {
	return &ast.BinaryExpr{
		X: &ast.SelectorExpr{
			X:   g.lexiconPkgIdent,
			Sel: ast.NewIdent(kind),
		},
		Op: token.AND,
		Y:  of,
	}
}

func (g *generator) refExpr(name string) (ast.Expr, error) {
	// TODO
	return &ast.IndexExpr{
		X:     ast.NewIdent("#D"),
		Index: stringLit(name),
	}, nil
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
