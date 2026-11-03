package runner

import (
	"maps"
	"slices"
)

// Visibility of a variable value.
type Visibility int

const (
	// VisibilityPublic values are always visible.
	VisibilityPublic Visibility = iota
	// VisibilitySecret values are redacted from standard error and reports.
	VisibilitySecret
)

// Variable is a named Value.
type Variable struct {
	Name       string
	Value      Value
	Visibility Visibility
}

// IsSecret reports whether this variable is secret.
func (v Variable) IsSecret() bool { return v.Visibility == VisibilitySecret }

// VariableSet is a set of variables, either injected at the start of the
// execution, or inserted during a run. The zero value is an empty set.
type VariableSet struct {
	variables map[string]Variable
	// secrets holds every secret value ever inserted: what's a secret
	// remains a secret forever, even if the variable gets a new value.
	secrets map[string]struct{}
}

// NewVariableSet returns an empty set of variables.
func NewVariableSet() *VariableSet {
	return &VariableSet{variables: map[string]Variable{}, secrets: map[string]struct{}{}}
}

// VariableSetFrom returns a set of public variables.
func VariableSetFrom(variables map[string]Value) *VariableSet {
	vs := NewVariableSet()
	for name, value := range variables {
		vs.Insert(name, value)
	}
	return vs
}

// Clone returns a copy of this set.
func (vs *VariableSet) Clone() *VariableSet {
	return &VariableSet{variables: maps.Clone(vs.variables), secrets: maps.Clone(vs.secrets)}
}

// Insert inserts a public variable.
func (vs *VariableSet) Insert(name string, value Value) {
	vs.init()
	vs.variables[name] = Variable{Name: name, Value: value, Visibility: VisibilityPublic}
}

// InsertSecret inserts a secret string variable, and registers its value
// as a secret.
func (vs *VariableSet) InsertSecret(name, value string) {
	vs.init()
	vs.secrets[value] = struct{}{}
	vs.variables[name] = Variable{Name: name, Value: ValueString{value}, Visibility: VisibilitySecret}
}

func (vs *VariableSet) init() {
	if vs.variables == nil {
		vs.variables = map[string]Variable{}
		vs.secrets = map[string]struct{}{}
	}
}

// Get returns the variable named name.
func (vs *VariableSet) Get(name string) (Variable, bool) {
	v, ok := vs.variables[name]
	return v, ok
}

// All returns all the variables, in no particular order (like Rust's
// HashMap iteration).
func (vs *VariableSet) All() map[string]Variable { return vs.variables }

// Secrets returns all the secret values, sorted.
func (vs *VariableSet) Secrets() []string {
	return slices.Sorted(maps.Keys(vs.secrets))
}
