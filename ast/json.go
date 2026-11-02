package ast

import "strings"

// JSONValue is the JSON used within Hurl (e.g. implicit JSON request bodies),
// a superset of JSON where strings are templates and placeholders may appear
// as values. It is one of JSONPlaceholder, JSONNumber, JSONString,
// JSONBoolean, JSONList, JSONObject or JSONNull.
type JSONValue interface {
	jsonValue()
	ToSource() string
}

type JSONPlaceholder struct{ Value Placeholder }
type JSONNumber struct{ Value string }
type JSONString struct{ Value Template }
type JSONBoolean struct{ Value bool }

type JSONList struct {
	Space0   string
	Elements []JSONListElement
}

type JSONObject struct {
	Space0   string
	Elements []JSONObjectElement
}

type JSONNull struct{}

func (JSONPlaceholder) jsonValue() {}
func (JSONNumber) jsonValue()      {}
func (JSONString) jsonValue()      {}
func (JSONBoolean) jsonValue()     {}
func (JSONList) jsonValue()        {}
func (JSONObject) jsonValue()      {}
func (JSONNull) jsonValue()        {}

// ToSource renders the placeholder's expression only, dropping the
// whitespace inside the braces (as Hurl does).
func (j JSONPlaceholder) ToSource() string { return "{{" + j.Value.String() + "}}" }
func (j JSONNumber) ToSource() string      { return j.Value }
func (j JSONString) ToSource() string      { return j.Value.ToSource() }

func (j JSONBoolean) ToSource() string {
	if j.Value {
		return "true"
	}
	return "false"
}

func (j JSONList) ToSource() string {
	elements := make([]string, len(j.Elements))
	for i, e := range j.Elements {
		elements[i] = e.ToSource()
	}
	return "[" + j.Space0 + strings.Join(elements, ",") + "]"
}

func (j JSONObject) ToSource() string {
	elements := make([]string, len(j.Elements))
	for i, e := range j.Elements {
		elements[i] = e.ToSource()
	}
	return "{" + j.Space0 + strings.Join(elements, ",") + "}"
}

func (JSONNull) ToSource() string { return "null" }

type JSONListElement struct {
	Space0 string
	Value  JSONValue
	Space1 string
}

func (e JSONListElement) ToSource() string {
	return e.Space0 + e.Value.ToSource() + e.Space1
}

type JSONObjectElement struct {
	Space0 string
	Name   Template
	Space1 string
	Space2 string
	Value  JSONValue
	Space3 string
}

func (e JSONObjectElement) ToSource() string {
	return e.Space0 + e.Name.ToSource() + e.Space1 + ":" + e.Space2 + e.Value.ToSource() + e.Space3
}
