# Progress

Status of the port, wave by wave. `git log` is the source of truth for what has landed.

## Done

- `reader`: rune cursor (hurl_core/reader.rs)
- `ast`: lossless AST, types, visitor (hurl_core/ast, types.rs). See `ast/doc.go` for the naming conventions.
- `internal/text`: styled strings and ANSI output (hurl_core/text)
- `internal/sourceerr`: annotated source error rendering (hurl_core/error)
- `internal/input`: file and stdin input (hurl_core/input.rs)
- `internal/jsonpath`: RFC 9535 JSONPath; the compliance suite gives 695 passed, 8 skipped, same as Rust
- `internal/html`: HTML escape and unescape, entity table
- `internal/pretty`: JSON pretty printer
- `internal/xpath`: XPath over antchfx/xpath with our own DOM (encoding/xml and x/net/html)

## Next

- `parser` (hurl_core/parser + combinator.rs)
- `internal/httpclient` (hurl/http) + logger
- then the hurlfmt formatter, runner, CLI, output, reports, parallel runner

## Notes for later porters

- `internal/pretty.Format` may write partial output when the JSON is invalid. The caller (the `runner/result.rs` port) must discard that output and write the raw bytes instead, as Rust does.
- `internal/jsonpath` copies Hurl 8.1.0 bugs on purpose, for example negative fractional literals (`-1.5` evaluates as `-0.5`).
- `internal/xpath`: numbers always map to a Hurl Float. Known HTML5 vs libxml2 tree differences are listed in the package doc.
- `internal/sourceerr`: implement `Messager` to override the default error message, as `RunnerError` needs.
