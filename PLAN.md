# go-hurl: Plan

A Go port of [Hurl](https://github.com/Orange-OpenSource/hurl), meant as a drop-in replacement for the `hurl` and `hurlfmt` binaries. Module path: `github.com/aviddiviner/go-hurl`.

## Reference version

We port from **Hurl 8.1.0** (tag `8.1.0`, commit `2641800`, released 2026-10-27). Upstream `master` is labelled 9.0.0, but it hasn't been released. It's 10 commits ahead of 8.1.0, and none of them changes behaviour:

- `cb6249d` Add `SourceInfo` to JSON AST values. This is a good idea for error positions and we'll likely do the same, but it doesn't change behaviour.
- `41d8d76`/`99a7642` Refactor `runner/assert` into a module and add `json_compare.rs`, which nothing calls yet (work in progress).
- Docs, version bumps, and a Python fix for the SSL test server.

## Goals

- **Drop-in replacement.** The binaries are named `hurl` and `hurlfmt`, take the same flags, and give the same exit codes, so existing deploy pipelines don't need to change.
- **Simple, self-contained, portable.** One static binary per platform, `CGO_ENABLED=0`, cross-compiled with plain `go build`.
- **Standard library first.** `golang.org/x/*` modules are fine where they restore a real feature. Any other third-party module has to justify itself.
- **Same file format.** `.hurl` files parse identically, and run the same way wherever we support the feature.
- **Not a goal: byte-for-byte parity.** Verbose output, header ordering, and curl-specific behaviour may differ. Where they do, we document it.

## Toolchain

We track the latest Go release: `go 1.26` in `go.mod`, matching the local toolchain. We'll bump as new releases come out.

## Dependencies

| Module | Why |
|---|---|
| `github.com/antchfx/xpath` | XPath 1.0 evaluation over our own `NodeNavigator` |
| `github.com/spf13/pflag` | curl-style flags (`-L`, `-v`, `--header`, short-flag bundling) |
| `golang.org/x/net/html` | Lenient HTML parsing for XPath on HTML bodies |
| `golang.org/x/text/encoding` | Charset support: `charsetDecode`, non-UTF-8 bodies (GB2312 etc.) via `htmlindex`, which uses WHATWG labels like Rust's `encoding_rs` |
| `golang.org/x/term` | Terminal width (progress bar, wrapping) |

## Rust crate → Go mapping

| Rust crate | Go |
|---|---|
| `curl`, `curl-sys` | `net/http`, `net/http/httptrace`, `crypto/tls`, custom `DialContext` |
| `libxml` (XPath, XML/HTML parsing) | `antchfx/xpath` over a small internal DOM, built with `encoding/xml` (XML) or `x/net/html` (HTML) |
| `libxml` (SAX: find the end of an XML body in the parser) | `encoding/xml` `Decoder.Token()` + `InputOffset()` |
| `clap` | `spf13/pflag` |
| `serde_json` (arbitrary precision) | `encoding/json` with `UseNumber()` + `math/big` for big integers |
| `regex` | `regexp` (RE2; note that `\d` and `\w` are ASCII-only in Go) |
| `chrono` | `time` + a hand-rolled strftime/strptime subset (chrono-style `%Y`, `%.f`, `%+`, …) |
| `encoding_rs` | `x/text/encoding/htmlindex` |
| `libflate` | `compress/gzip`, `compress/flate`, `compress/zlib` |
| `brotli` | **dropped** |
| `glob` | `path/filepath.Glob` + a small `**` walker (`filepath.WalkDir` + `path.Match`) |
| `similar` | Hand-rolled Myers line diff (~150 lines) |
| `md5`, `sha2`, `base64`, `percent-encoding`, `url` | `crypto/md5`, `crypto/sha256`, `encoding/base64`, `net/url` |
| `uuid` | `crypto/rand` (v4 is ~10 lines) |
| `xml` (JUnit writer) | `encoding/xml` |
| `colored`, `terminal_size` | ANSI escapes, `x/term` |

## Identity

Hurl sends `User-Agent: hurl/<version>` and prints `hurl <version> (<target>) <libcurl info>` for `--version`. Proposal:

- **User-Agent:** `hurl/8.1.0 go-hurl/0.1.0`. These are two RFC 9110 product tokens. The first says which Hurl version we target, and the second identifies us. This still matches anything that checks for a `hurl/` prefix, including all 130 upstream integration patterns (`User-Agent: hurl/<<<.*?>>>`).
- **`--version`:** for example `hurl 8.1.0 (go-hurl 0.1.0) go1.26.6 darwin/arm64`, with a `Features:` line listing what we support. Putting the Hurl version we target first keeps version checks in existing pipelines working.
- **go-hurl versioning:** our own semver, starting at `0.1.0`, independent of Hurl's.

## Licensing

Hurl is Apache-2.0, and so is go-hurl. My understanding of what that requires (not legal advice):

- `LICENSE`: Hurl's Apache-2.0 text, copied verbatim.
- `NOTICE`: credits Hurl and Orange, says the Go code is translated and modified from Hurl 8.1.0, and lists the files copied unmodified. Hurl has no `NOTICE` of its own to carry forward.
- Ported Go files don't need per-file headers. The `NOTICE` covers the "you must state that you changed the files" requirement for a translation.
- The license grants no trademark rights. The README should say plainly that go-hurl is an independent port, not affiliated with or endorsed by Orange.

## Feature matrix

### Kept

- The whole `.hurl` grammar: entries, sections, options, templates, functions (`newDate`, `newUuid`), JSON/XML/multiline/base64/hex/file bodies, multipart, GraphQL.
- Captures, asserts and all predicates; queries: `status`, `version`, `url`, `header`, `cookie`, `body`, `bytes`, `rawbytes`, `xpath`, `jsonpath`, `regex`, `variable`, `duration`, `sha256`, `md5`, `certificate`, `ip`, `redirects`.
- All filters, including `charsetDecode`.
- JSONPath (RFC 9535), ported from Hurl's hand-written implementation and validated against its compliance suite.
- HTTP/1.1 and HTTP/2 (TLS ALPN), plus h2c prior knowledge (`Transport.Protocols`).
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
| Brotli (`br`) response decoding | no stdlib or `x/` decoder; `--compressed` sends `gzip, deflate` only |
| `--http1.0` / HTTP/1.0 requests | `net/http` always sends HTTP/1.1 |
| Encrypted client keys (`--cert file:password`) | stdlib has no PKCS#8 decryption, and the legacy PEM encryption it does have is deprecated |
| `--ssl-no-revoke` | Windows/Schannel-only; accepted and ignored |
| libcurl's `**` connection chatter in `--very-verbose` | replaced by our own lines built from `httptrace` events |

Dropped flags are still **accepted**, but fail with a clear "not supported by go-hurl" error. That way an unsupported pipeline fails loudly instead of quietly doing something different.

### Known differences

- Request headers are sent (and printed) in Go's sorted order, not file order.
- URL parsing follows `net/url` (RFC 3986), not WHATWG, so edge cases may normalise differently.
- HTML trees come from an HTML5 parser (`x/net/html`, which inserts implied `<tbody>` etc.), not libxml2's HTML4-era parser. XPath on HTML can differ on markup that relies on those rules.
- Response header names that `net/http` rejects (e.g. `<script>: x`) are accepted like curl does on plain-text HTTP/1.x only; over TLS the transfer fails.

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
  integration/         Hurl 8.1.0 integration suite, copied unmodified
  bin/                 Hurl 8.1.0 test-server scripts, copied unmodified
```

`ast` and `parser` are public; everything else is `internal/`.

## Parser approach

Hurl's parser is scannerless recursive descent. A `Reader` holds the input as `Vec<char>` with a cursor (`index` plus `line`/`column`), and parse functions are plain `fn(&mut Reader) -> Result<T, ParseError>`. The combinators (`optional`, `zero_or_more`, `one_or_more`, `choice`, `recover`, `non_recover`) backtrack by saving and restoring the cursor. Errors are either *recoverable* (try the next alternative) or *fatal* (stop and report).

We **follow this structure closely**, for these reasons:

- The grammar depends on context throughout. What counts as a token after `HTTP 200` is different from inside a `[Captures]` line, a header value (an unquoted template to end of line), a JSON body with `{{ }}` placeholders, or a backtick multiline string. A separate token stream in the style of Pike's `lex` would need a lot of lexer states that mirror the parser anyway.
- Matching Hurl's error positions and messages is much easier when the code has the same shape. The `tests_error_parser` corpus checks those messages exactly.
- It maps neatly onto Go: `Reader` holds a `[]rune` (columns stay in characters, as in Rust), and Go generics give us the combinators directly (`func optional[T any](r *Reader, f func(*Reader) (T, error)) (*T, error)`).

The Pike-style state-function lexer is still a good fit for small, self-contained sub-languages if we want it: JSONPath expressions, the curl command-line import, or config-file tokenising. (Pike's own `text/template` ended up dropping the goroutine for a synchronous `nextItem` loop, which is the variant I'd use.)

Rust enums become Go sealed interfaces (an unexported marker method) with type switches. Struct-like variants become small structs. The AST keeps Hurl's lossless shape (`space0`, `line_terminators`, comments) so that `hurlfmt` can round-trip files.

## Testing strategy

The upstream integration suite is copied into `integration/` and `bin/` with the same layout, so its scripts run unchanged (`bin/test/test_prerequisites.sh` starts the Flask, SSL, unix-socket and squid servers, then `cd integration/hurl && ./integration.py`).

1. **Unit tests:** table-driven, ported module by module from the Rust `#[cfg(test)]` blocks (~25k lines to draw from).
2. **Parser corpus:** every `.hurl` file in `integration/hurl/tests_ok*` must parse. Every file in `tests_error_parser` must fail with the expected message.
3. **Round-trip:** for every linted file in `tests_ok`, `format(parse(src)) == src`. This is a strong check that the AST really is lossless.
4. **JSONPath compliance:** add `cts.json` to `internal/jsonpath/testdata/` in phase 4.
5. **Integration:** build our binaries, put them first on `PATH`, and run the upstream scripts. We'll keep a skip list for dropped features and known differences, and that list doubles as documentation. It lives outside `integration/` so the copied files stay unmodified.

## Phases

| Phase | Scope | Done when |
|---|---|---|
| 0 | `go.mod`, README, CI (`go vet`, `go test`, cross-build matrix) | `go build ./...` is green on linux/darwin/windows |
| 1 | `ast`, `parser`, `sourceerr`, `text` | parser corpus + `tests_error_parser` pass |
| 2 | `hurlfmt`: Hurl and JSON output, linter, then HTML output and curl import | round-trip test passes; `integration/hurlfmt` passes |
| 3 | `httpclient`: request/response, redirects, cookies, TLS, proxy, timings, decoding | unit tests against `httptest.Server` |
| 4 | `runner`, `jsonpath`, `xpath`: templates, captures, queries, filters, predicates, asserts, runtime errors | `hurl --test` works on simple files; JSONPath CTS passes |
| 5 | `cli` + `output`: all retained flags, config file, env vars, verbose output, `--curl`, `--json` | majority of `tests_ok` integration scripts pass |
| 6 | `report`, `parallel`, progress bar | report and parallel integration tests pass |
| 7 | Conformance sweep: shrink the skip list, write up differences | skip list contains only dropped or known-difference items |

Phases 1–2 can be done with no network code at all. Phase 3 can run in parallel with phase 2 if we want.

## Open questions

1. **Identity:** are you happy with the User-Agent and `--version` scheme above?
2. **Dropped flags:** should they fail loudly (proposed), or warn and continue?
