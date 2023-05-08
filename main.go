package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	_ "embed"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
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
		if err := checkLexSchema(w.Path(), lexiconSchema); err != nil {
			fmt.Printf("%s: %v\n", w.Path(), err)
			return
		}
		n++
	}
	fmt.Printf("%d schemas checked\n", n)
}

func checkLexSchema(f string, lexiconSchema cue.Value) error {
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
	return nil
}
