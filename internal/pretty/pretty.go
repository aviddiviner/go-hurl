// Package pretty prettifies response bodies.
package pretty

// PrettyMode selects when response bodies are prettified.
type PrettyMode int

const (
	// Automatic prettifies based on the response content type.
	Automatic PrettyMode = iota
	// Force tries to prettify even if there is no Content-Type response header.
	Force
	// None disables prettifying.
	None
)
