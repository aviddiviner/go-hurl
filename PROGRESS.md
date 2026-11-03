# Progress

Status of the port. `git log` is the source of truth for what has landed.

## Status

The whole port is written: `hurl` and `hurlfmt` build as static binaries (`CGO_ENABLED=0`) for linux, darwin and windows. `go vet ./...` and `go test ./...` pass.

Upstream integration suite (`scripts/integration.py`, local run without squid): **399 passed, 8 failed, 24 skipped**. The 8 failures are proxy tests, which need squid on port 3128. The skips are listed with reasons in `scripts/integration_skip.txt`. The hurlfmt integration suite passes in full.

| Package | Ported from |
|---|---|
| `reader`, `ast`, `parser` | hurl_core reader, ast, types, parser, combinator |
| `internal/text`, `sourceerr`, `input` | hurl_core text, error, input |
| `internal/format`, `lint`, `curlimport`, `cmd/hurlfmt` | hurl_core/format, hurlfmt |
| `internal/jsonpath` | hurl/jsonpath (CTS: 695 passed, 8 skipped, same as Rust) |
| `internal/xpath` | libxml2 XPath → antchfx/xpath + own DOM |
| `internal/html`, `pretty` | hurl/html, hurl/pretty |
| `internal/httpclient` | hurl/http, libcurl → net/http |
| `internal/logger`, `term`, `util` | hurl/util, runner/output.rs |
| `internal/runner`, `timefmt` | hurl/runner, hurl/json; chrono formatting |
| `internal/regex` | Rust regex-syntax checks on top of Go `regexp` |
| `internal/report/...` | hurl/report (curl, html, json, junit, tap) |
| `internal/cli`, `output`, `parallel`, `cmd/hurl` | hurl/cli, output, parallel, main.rs, run.rs |

## Next

- Run the proxy tests with squid (CI does this: `.github/workflows/ci.yml`).
- Deferred features from `PLAN.md`: `--aws-sigv4`, `--digest`, shell completions.
- A review pass over the larger packages (httpclient, runner, cli).

## Notes for later porters

- `internal/jsonpath` and `internal/runner` copy some Hurl 8.1.0 bugs on purpose, with comments (e.g. negative fractional literals in JSONPath, `-1.5` displayed as `-1.5.0`).
- `internal/runner/diff*.go` reproduces the `similar` 3.2.1 crate (Myers + compaction). It is checked against 3000 cases generated with the real crate (`testdata/similar_diffs.*`). Large-input heuristics aren't ported.
- `internal/httpclient` has a few curl-isms built on net/http: CN hostname fallback for certificates without SANs, lenient parsing of invalid response header names (plain HTTP only), no `Content-Length: 0` on empty POSTs, curl's cookie sort order.
- `scripts/integration_normalize.py` makes the integration comparison ignore header order (a known difference).
