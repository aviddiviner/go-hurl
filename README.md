# go-hurl

A Go port of [Hurl](https://hurl.dev): run and test HTTP requests written in a plain text format. go-hurl aims to be a drop-in replacement for the `hurl` and `hurlfmt` binaries, built as a single static Go binary with no cgo, no libcurl and no libxml2.

go-hurl targets **Hurl 8.1.0**. It is an independent port and is not affiliated with or endorsed by Orange or the Hurl project.

## Install

Prebuilt binaries are on the [releases page](https://github.com/aviddiviner/go-hurl/releases), packaged like upstream Hurl's: `hurl-<version>-<target>.tar.gz` (with `hurl-<version>-<target>/bin/hurl` and `hurlfmt` inside) plus a `.sha256` for each, for `x86_64`/`aarch64` `unknown-linux-gnu` and `apple-darwin`, and `x86_64-pc-windows-msvc` (zip). Release tags have a `v` prefix (`v0.1.0`), while archive names don't:

```sh
VERSION=0.1.0 target=x86_64-unknown-linux-gnu
curl -sSfLO "https://github.com/aviddiviner/go-hurl/releases/download/v$VERSION/hurl-$VERSION-$target.tar.gz"
```

Or with Go:

```sh
go install github.com/aviddiviner/go-hurl/cmd/hurl@latest
go install github.com/aviddiviner/go-hurl/cmd/hurlfmt@latest
```

Or build from a checkout: `CGO_ENABLED=0 go build -o build/ ./cmd/...`.

To cut a release, bump `GoHurl` in `internal/version/version.go`, commit, then push a `v<version>` tag. The release workflow checks the tag matches, runs the tests, builds the archives with `scripts/package.sh` and publishes them.

## Usage

The same as Hurl. See the [Hurl documentation](https://hurl.dev/docs/manual.html).

```hurl
GET https://example.org/api/health
HTTP 200
[Asserts]
jsonpath "$.status" == "ok"
```

```sh
hurl --test health.hurl
```

`hurl --version` prints the targeted Hurl version first (`hurl 8.1.0 (go-hurl 0.1.0) ...`), and the default User-Agent is `hurl/8.1.0 go-hurl/0.1.0`.

## Differences from Hurl

The `.hurl` file format is fully supported. HTTP is done with Go's `net/http` instead of libcurl, and XPath with [antchfx/xpath](https://github.com/antchfx/xpath) instead of libxml2, so a few things differ.

**Not supported** (the flag or option is accepted, but fails with a "not supported by go-hurl" error): `--http3`, `--http1.0`, `--ntlm`, `--negotiate`, `--digest`, `--aws-sigv4`, and client certificates with a password. Brotli responses fail to decode, so `--compressed` asks for `gzip, deflate` only. `--ssl-no-revoke` is accepted and ignored.

**Behaves differently:**
- Request headers are sent in Go's order, not file order.
- URLs are parsed per RFC 3986 (`net/url`), not WHATWG.
- HTML is parsed with an HTML5 parser, so XPath results can differ on malformed markup.
- `--very-verbose` connection logging (`**` lines) is go-hurl's own, not libcurl's.

`PLAN.md` has the full feature matrix.

## Development

```sh
go test ./...
```

The upstream integration suite is in `integration/` (copied unmodified from Hurl 8.1.0). It needs Python 3 with the packages in `bin/requirements-frozen.txt` and a squid proxy. To run it against go-hurl:

```sh
CGO_ENABLED=0 go build -o build/ ./cmd/...
bin/test/test_prerequisites.sh            # starts the test servers
PATH=$PWD/build:$PATH python3 scripts/integration.py
```

`scripts/integration.py` keeps going after failures and skips the scripts listed in `scripts/integration_skip.txt`.

## License

Apache-2.0, like Hurl. See `LICENSE` and `NOTICE`.
