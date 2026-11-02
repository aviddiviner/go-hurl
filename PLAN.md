# go-hurl: Plan

A Go port of [Hurl](https://github.com/Orange-OpenSource/hurl) (reference checkout in `../hurl`, v9.0.0). This document is for discussion before we start building.

## Goals

- **Simple, self-contained, portable.** One static binary per platform, `CGO_ENABLED=0`, cross-compiles with plain `go build`.
- **Standard library first.** Each third-party dependency has to justify itself. A feature that would need a non-trivial dependency gets dropped (or deferred) rather than pulling one in.
- **Same file format.** `.hurl` files written for Hurl should parse identically, and run the same way wherever we support the feature.
- **Not a goal: byte-for-byte parity.** Verbose output, header ordering, and curl-specific behaviour may differ. Where they do, we document it.

## Dependency budget

| Module | Why | Transitive deps |
|---|---|---|
| `github.com/antchfx/xpath` | XPath 1.0 evaluation. Writing this ourselves isn't worth it. | none |
| `github.com/spf13/pflag` | curl-style flags (`-L`, `-v`, `--header`, short-flag bundling). | none |

That's the whole list. Everything else is the standard library, including some deliberately small hand-rolled pieces (see below). `golang.org/x/*` modules are **not** included by default. Each one would buy back a specific feature (see Open questions).

## Rust crate → Go mapping

| Rust crate | Go |
|---|---|
| `curl`, `curl-sys` | `net/http`, `net/http/httptrace`, `crypto/tls`, custom `DialContext` |
| `libxml` (XPath, XML/HTML parsing) | `antchfx/xpath` + our own small DOM and `NodeNavigator` built on `encoding/xml` |
| `libxml` (SAX: find the end of an XML body in the parser) | `encoding/xml` `Decoder.Token()` + `InputOffset()` |
| `clap` | `spf13/pflag` |
| `serde_json` (arbitrary precision) | `encoding/json` with `UseNumber()` + `math/big` for big integers |
| `regex` | `regexp` (RE2; note that `\d` and `\w` are ASCII-only in Go) |
| `chrono` | `time` + a hand-rolled strftime/strptime subset (chrono-style `%Y`, `%.f`, `%+`, …) |
| `encoding_rs` | Hand-rolled: UTF-8, US-ASCII, ISO-8859-1, UTF-16LE/BE. Other charsets dropped. |
| `libflate` | `compress/gzip`, `compress/flate`, `compress/zlib` |
| `brotli` | **dropped** |
| `glob` | `path/filepath.Glob` + a small `**` walker (`filepath.WalkDir` + `path.Match`) |
| `similar` | Hand-rolled Myers line diff (~150 lines) |
| `md5`, `sha2`, `base64`, `percent-encoding`, `url` | `crypto/md5`, `crypto/sha256`, `encoding/base64`, `net/url` |
| `uuid` | `crypto/rand` (v4 is ~10 lines) |
| `xml` (JUnit writer) | `encoding/xml` |
| `colored`, `terminal_size` | ANSI escapes. TTY detection via `os.File.Stat()`. Width from `COLUMNS`, defaulting to 80. |

### HTML for XPath

Hurl uses libxml2's lenient HTML parser. We'll use `encoding/xml` in non-strict mode (`Strict = false`, `AutoClose = xml.HTMLAutoClose`, `Entity = xml.HTMLEntity`) to build the same DOM we use for XML. That handles well-formed and mildly sloppy HTML. On badly broken markup the tree may differ from libxml2's. If that becomes a problem, `golang.org/x/net/html` is the escape hatch.

## Feature matrix

### Kept (stdlib)

- The whole `.hurl` grammar: entries, sections, options, templates, functions (`newDate`, `newUuid`), JSON/XML/multiline/base64/hex/file bodies, multipart, GraphQL.
- Captures, asserts and all predicates; queries: `status`, `version`, `url`, `header`, `cookie`, `body`, `bytes`, `rawbytes`, `xpath`, `jsonpath`, `regex`, `variable`, `duration`, `sha256`, `md5`, `certificate`, `ip`, `redirects`.
- All filters, with `charsetDecode` limited to the encodings above.
- JSONPath (RFC 9535), ported from Hurl's hand-written implementation and validated against its compliance suite.
- HTTP/1.1 and HTTP/2 (TLS ALPN), plus h2c prior knowledge (`Transport.Protocols`, Go 1.24+).
- Hurl's own redirect handling (`CheckRedirect` → `http.ErrUseLastResponse`), `--location-trusted`, `--max-redirs`.
- Retry, delay, repeat, skip, `--from-entry`/`--to-entry`, `--continue-on-error`, `--test`, `--parallel`/`--jobs`.
- `--resolve`, `--connect-to`, `--unix-socket`, `--ipv4`/`--ipv6`, `--connect-timeout`, `--max-time`.
- TLS: `--insecure`, `--cacert`, `--cert`/`--key` (unencrypted PEM), `--tls-min`/`--tls-max`, `--pinnedpubkey`, certificate queries.
- `--proxy`, `--noproxy`, `--proxy-header`.
- `--user` (basic), `--netrc`/`--netrc-file`/`--netrc-optional`.
- Cookies: our own jar with Netscape-format `--cookie`/`--cookie-jar` (`net/http/cookiejar` can't list or export).
- `--compressed` (gzip, deflate), `--limit-rate`, `--max-filesize`, `--path-as-is`.
- Timings via `httptrace`, `--curl` export, `--json`, `--output`, `--include`, colour, secrets redaction, variables, `--variables-file`, `HURL_*` env vars, config file.
- Reports: JSON, JUnit, TAP, HTML (`html/template` + `embed`).
- `hurlfmt`: format to Hurl/JSON/HTML, linter, `--in curl` import.

### Deferred (doable with stdlib, just not early)

- `--aws-sigv4`: it's HMAC-SHA256 over a canonical request, about 200 lines.
- `--digest`: an MD5/SHA-256 challenge-response.
- Shell completions.

### Dropped

| Feature | Reason |
|---|---|
| `--http3` | needs `quic-go` |
| `--ntlm`, `--negotiate` | needs NTLM/Kerberos libraries |
| Brotli (`br`) response decoding | no stdlib decoder; `--compressed` sends `gzip, deflate` only |
| Charsets other than UTF-8/ASCII/Latin-1/UTF-16 (e.g. GB2312) | needs `golang.org/x/text` |
| `--http1.0` / HTTP/1.0 requests | `net/http` always sends HTTP/1.1 |
| Encrypted client keys (`--cert file:password`) | stdlib has no PKCS#8 decryption, and the legacy PEM encryption it does have is deprecated |
| `--ssl-no-revoke` | Windows/Schannel-only; accepted and ignored |
| libcurl's `**` connection chatter in `--very-verbose` | replaced by our own lines built from `httptrace` events |

### Known differences

- Request headers are sent (and printed) in Go's sorted order, not file order.
- URL parsing follows `net/url` (RFC 3986), not WHATWG, so edge cases may normalise differently.
- XPath on badly malformed HTML (see above).

## Package layout

```
go-hurl/
  cmd/hurl/            main for hurl
  cmd/hurlfmt/         main for hurlfmt
  ast/                 hurl_core/ast: lossless AST (keeps whitespace and comments for hurlfmt)
  parser/              hurl_core/parser + reader + combinator
  internal/
    text/              hurl_core/text: styled strings, ANSI rendering
    sourceerr/         hurl_core/error: source-annotated error display (shared by parser and runner)
    format/            hurl_core/format + hurlfmt/format: Hurl, JSON, HTML output
    lint/              hurlfmt/linter
    curlimport/        hurlfmt/curl
    jsonpath/          hurl/jsonpath
    xpath/             DOM + antchfx navigator
    httpclient/        hurl/http
    runner/            hurl/runner: values, templates, queries, filters, predicates, asserts
    report/            hurl/report: json, junit, tap, html
    cli/               hurl/cli: options, config file, env, variables
    output/, pretty/, parallel/
```

`ast` and `parser` are public because they're useful on their own (editors, linters, other tools). Everything else stays `internal/` until there's a reason to expose it.

## Parser approach

Hurl's parser is scannerless recursive descent. A `Reader` holds the input as `Vec<char>` with a cursor (`index` plus `line`/`column`), and parse functions are plain `fn(&mut Reader) -> Result<T, ParseError>`. The combinators (`optional`, `zero_or_more`, `one_or_more`, `choice`, `recover`, `non_recover`) backtrack by saving and restoring the cursor. Errors are either *recoverable* (try the next alternative) or *fatal* (stop and report).

I propose we **follow this structure closely**, for these reasons:

- The grammar depends on context throughout. What counts as a token after `HTTP 200` is different from inside a `[Captures]` line, a header value (an unquoted template to end of line), a JSON body with `{{ }}` placeholders, or a backtick multiline string. A separate token stream in the style of Pike's `lex` would need a lot of lexer states that mirror the parser anyway.
- Matching Hurl's error positions and messages is much easier when the code has the same shape. The `tests_error_parser` corpus checks those messages exactly.
- It maps neatly onto Go: `Reader` holds a `[]rune` (columns stay in characters, as in Rust), and Go generics give us the combinators directly (`func optional[T any](r *Reader, f func(*Reader) (T, error)) (*T, error)`).

The Pike-style state-function lexer is still a good fit for small, self-contained sub-languages if we want it: JSONPath expressions, the curl command-line import, or config-file tokenising. (Pike's own `text/template` ended up dropping the goroutine for a synchronous `nextItem` loop, which is the variant I'd use.)

Rust enums become Go sealed interfaces (an unexported marker method) with type switches. Struct-like variants become small structs. The AST keeps Hurl's lossless shape (`space0`, `line_terminators`, comments) so that `hurlfmt` can round-trip files.

## Testing strategy

1. **Unit tests:** table-driven, ported module by module from the Rust `#[cfg(test)]` blocks (~25k lines to draw from).
2. **Parser corpus:** every `.hurl` file in `../hurl/integration/hurl/tests_ok*` must parse. Every file in `tests_error_parser` must fail with the expected message.
3. **Round-trip:** for every linted file in `tests_ok`, `format(parse(src)) == src`. This is a strong check that the AST really is lossless.
4. **JSONPath compliance:** run `jsonpath/tests/cts.json`.
5. **Integration:** run Hurl's own integration scripts (`.sh` + `.out`/`.err`/`.pattern`/`.exit`) with our binaries first on `PATH`, against its Flask test server. We'll keep a skip list for dropped features and known differences, and that list doubles as documentation.

## Phases

| Phase | Scope | Done when |
|---|---|---|
| 0 | `go.mod`, LICENSE (Apache-2.0) + NOTICE crediting Orange, README, CI (`go vet`, `go test`, cross-build matrix) | `go build ./...` is green on linux/darwin/windows |
| 1 | `ast`, `parser`, `sourceerr`, `text` | parser corpus + `tests_error_parser` pass |
| 2 | `hurlfmt`: Hurl and JSON output, linter, then HTML output and curl import | round-trip test passes |
| 3 | `httpclient`: request/response, redirects, cookies, TLS, proxy, timings, decoding | unit tests against `httptest.Server` |
| 4 | `runner`, `jsonpath`, `xpath`: templates, captures, queries, filters, predicates, asserts, runtime errors | `hurl --test` works on simple files; JSONPath CTS passes |
| 5 | `cli` + `output`: all retained flags, config file, env vars, verbose output, `--curl`, `--json` | majority of `tests_ok` integration scripts pass |
| 6 | `report`, `parallel`, progress bar | report and parallel integration tests pass |
| 7 | Conformance sweep: shrink the skip list, write up differences | skip list contains only dropped or known-difference items |

Phases 1–2 can be done with no network code at all. Phase 3 can run in parallel with phase 2 if we want.

## Open questions

1. **Module path:** `github.com/<who>/go-hurl`?
2. **`golang.org/x/*` modules:** each one buys back a specific feature: `x/text` gives full charset support (GB2312 etc.), `x/net/html` gives libxml-like HTML trees for XPath, `x/term` gives the real terminal width. My lean: none to start, and revisit per feature.
3. **Public API:** happy with `ast` and `parser` public and everything else internal?
4. **Go version floor:** 1.24 is the minimum for `Transport.Protocols`/h2c; 1.26 is what's installed locally.
5. **Integration assets:** reference `../hurl/integration` in place, or copy the test files into `testdata/`? Referencing them avoids drift while we're porting; copying makes the repo self-contained.
6. **Identity:** the binary is called `hurl` (drop-in) or `go-hurl`? What `User-Agent`/`--version` string?
