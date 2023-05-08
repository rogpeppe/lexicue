package main

import "strings"

type Schema struct {
	prefix string

	Lexicon int                    `json:"lexicon"`
	ID      string                 `json:"id"`
	Defs    map[string]*TypeSchema `json:"defs"`
}

// TODO(bnewbold): suspect this param needs updating for lex refactors
type Param struct {
	Type     string `json:"type"`
	Maximum  int    `json:"maximum"`
	Required bool   `json:"required"`
}

type OutputType struct {
	Encoding string      `json:"encoding"`
	Schema   *TypeSchema `json:"schema"`
}

type InputType struct {
	Encoding string      `json:"encoding"`
	Schema   *TypeSchema `json:"schema"`
}

type TypeSchema struct {
	Type        string      `json:"type"`
	Key         string      `json:"key"`
	Description string      `json:"description"`
	Parameters  *TypeSchema `json:"parameters"`
	Input       *InputType  `json:"input"`
	Output      *OutputType `json:"output"`
	Record      *TypeSchema `json:"record"`

	Ref        string                 `json:"ref"`
	Refs       []string               `json:"refs"`
	Required   []string               `json:"required"`
	Nullable   []string               `json:"nullable"`
	Properties map[string]*TypeSchema `json:"properties"`
	MaxLength  int                    `json:"maxLength"`
	Items      *TypeSchema            `json:"items"`
	Const      any                    `json:"const"`
	Enum       []string               `json:"enum"`
	Closed     bool                   `json:"closed"`

	Default any `json:"default"`
	Minimum any `json:"minimum"`
	Maximum any `json:"maximum"`
}

func (s *Schema) Name() string {
	p := strings.Split(s.ID, ".")
	return p[len(p)-2] + p[len(p)-1]
}
