package runner

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
)

// evalFunction evaluates a Hurl function.
func evalFunction(f ast.Function) (Value, *RunnerError) {
	switch f {
	case ast.FunctionNewDate:
		return ValueDate{time.Now().UTC()}, nil
	case ast.FunctionNewUUID:
		return ValueString{newUUIDv4()}, nil
	}
	panic("unknown function " + f.String())
}

func newUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
