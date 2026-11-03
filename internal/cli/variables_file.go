package cli

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/internal/runner"
)

// readVariablesFile reads a variables file of "name=value" lines (blank
// lines and # comments are skipped) and calls fn for each variable.
func readVariablesFile(path string, kind typeKind, fn func(name string, value runner.Value) error) error {
	if _, err := os.Stat(path); err != nil {
		return optionsErrorf("Variables file %s does not exist", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return optionsErrorf("Error opening %s", path)
	}
	defer f.Close()
	r := bufio.NewReader(f)
	for index := 0; ; index++ {
		line, err := r.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return optionsErrorf("Can not parse line %d of %s", index+1, path)
		}
		line = bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
		if !utf8.Valid(line) {
			return optionsErrorf("Can not parse line %d of %s", index+1, path)
		}
		s := strings.TrimSpace(string(line))
		if strings.HasPrefix(s, "#") || s == "" {
			continue
		}
		name, value, perr := parseVariable(s, kind)
		if perr != nil {
			return perr
		}
		if err := fn(name, value); err != nil {
			return err
		}
	}
}
