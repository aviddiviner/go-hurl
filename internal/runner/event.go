package runner

import "github.com/aviddiviner/go-hurl/ast"

// EventListener observes the run of one Hurl file.
type EventListener interface {
	// OnEntryRunning is called when running an entry. current is the entry
	// index in the Hurl file, last is the last entry index (may be less than
	// the total number of entries) and retryCount is the current number of
	// retries (0 for a first run).
	OnEntryRunning(current, last ast.Index, retryCount int)
}
