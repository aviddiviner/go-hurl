# Porting conventions

Rules for porting Hurl's Rust code to go-hurl, read by anyone (human or agent) working on the port. See `PLAN.md` for scope and decisions, and `PROGRESS.md` for status.

## Reference

- The Rust reference is `../hurl` (i.e. `/Users/dave/dev/public/hurl`), checked out at tag `8.1.0`. Read it, never modify it.
- Port behaviour faithfully: same parse results, same error kinds and positions, same messages, same output text. When a Go library forces a difference, keep it minimal and note it in your final report.
- `integration/` and `bin/` are copied verbatim from upstream. Never edit them.

## Go style

- Go 1.26, module `github.com/aviddiviner/go-hurl`. Use modern Go: generics, `min`/`max`, `range` over ints, `slices`/`maps`/`strings` helpers.
- Keep code plain and idiomatic. No per-file license headers. Doc comments only on exported identifiers where they add something; no comments that restate code.
- Prefer the standard library. Allowed third-party modules: `github.com/antchfx/xpath`, `github.com/spf13/pflag`, `golang.org/x/net`, `golang.org/x/text`, `golang.org/x/term`. Ask before adding anything else.
- `gofmt` clean and `go vet ./...` clean.

## Mapping Rust to Go

- **Structs** → structs with exported fields, `snake_case` → `CamelCase` (`line_terminator0` → `LineTerminator0`, `source_info` → `SourceInfo`).
- **`Option<T>`** → `*T` for structs, or a zero value when it is unambiguous (e.g. an empty slice for `Vec`).
- **C-like enums** (no payloads) → `type X int` with `const ( ... iota )` and a `String()` method when Rust has `Display`.
- **Enums with payloads** → a sealed interface (an unexported marker method) plus one struct per variant, consumed with type switches. Name variant types `<Enum><Variant>` when the bare variant name would clash or be unclear (e.g. `QueryValue::Header` → `QueryHeader`). Unit variants become empty structs.
- **`Display`** → `String() string`. **`ToSource`** → `ToSource() string`.
- **`Result<T, E>`** → `(T, error)`, or `(T, *SpecificError)` where callers need the concrete type (the parser does).
- **`Vec<char>`/`char`** → `[]rune`/`rune`. Positions and columns count runes, as in Rust.
- **`u64`/`i64`/`usize`** → `uint64`/`int64`/`int`.

## Tests

- Port the Rust `#[cfg(test)]` tests for each module you port, as table-driven Go tests in the same package. Take expected values from the Rust tests; don't compute them with the code under test.
- Before finishing: `go vet ./...` and `go test ./...` must pass.

## Working in parallel

- Several people or agents may be working in the repo at once. Only write inside the packages assigned to you.
- Don't run `git commit`, `git add`, `go mod tidy`, or anything else that touches the git index. If you need a dependency, run `go get <module>@<version>` and say so in your report.
