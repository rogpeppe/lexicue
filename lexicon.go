package main

import "strings"

type Schema struct {
	Lexicon int                    `json:"lexicon"`
	ID      string                 `json:"id"`
	Defs    map[string]*TypeSchema `json:"defs"`
}

type TypeSchema struct {
	// all
	Type        string `json:"type"`
	Description string `json:"description"`

	// record
	Key    string      `json:"key,omitempty"`
	Record *TypeSchema `json:"record"`

	// subscription, query
	Parameters *TypeSchema `json:"parameters"`
	Message *SubscriptionMessage	`json:"message"`

	// procedure
	Input *BodyType `json:"input"`

	// query, procedure
	Output *BodyType `json:"output"`

	// ref
	Ref string `json:"ref"`

	// union
	Refs []string `json:"refs"`

	// object, params
	Required []string `json:"required"`

	// object
	Nullable []string `json:"nullable"`

	// object, params
	Properties map[string]*TypeSchema `json:"properties"`

	// array, string
	MinLength *int `json:"minLength,omitempty"`

	// video, audio, array, string, bytes
	MaxLength *int `json:"maxLength,omitempty"`

	// array
	Items *TypeSchema `json:"items"`

	// bool, number, integer, string
	Const any `json:"const,omitempty"`

	// number, integer, string,
	Enum []any `json:"enum"`

	// union
	Closed bool `json:"closed"`

	// bool, number, integer, string
	Default any `json:"default,omitempty"`

	// number, integer
	Minimum any `json:"minimum"`
	Maximum any `json:"maximum"`

	// string
	MaxGraphemes *int     `json:"maxGraphemes,omitempty"`
	KnownValues  []string `json:"knownValues,omitempty"`

	// procedure, query, subscription
	Errors []Error `json:"errors,omitempty"`
}

type SubscriptionMessage struct {
	Schema *TypeSchema 	`json:"schema"`
}

type Error struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// TODO(bnewbold): suspect this param needs updating for lex refactors
type Param struct {
	Type     string `json:"type"`
	Maximum  int    `json:"maximum"`
	Required bool   `json:"required"`
}

type BodyType struct {
	Encoding string      `json:"encoding"`
	Schema   *TypeSchema `json:"schema"`
}

func (s *Schema) Name() string {
	p := strings.Split(s.ID, ".")
	return p[len(p)-2] + p[len(p)-1]
}
