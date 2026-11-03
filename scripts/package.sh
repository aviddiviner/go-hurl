#!/bin/bash
# Builds release archives laid out like upstream Hurl's:
#   hurl-<version>-<target>.tar.gz containing hurl-<version>-<target>/bin/{hurl,hurlfmt}
#   hurl-<version>-x86_64-pc-windows-msvc.zip containing hurl.exe, hurlfmt.exe
# plus a .sha256 next to each archive.
#
#   scripts/package.sh <version> [target ...]
set -Eeuo pipefail

version="${1:?usage: scripts/package.sh <version> [target ...]}"
shift
targets=("$@")
if [ ${#targets[@]} -eq 0 ]; then
  targets=(
    x86_64-unknown-linux-gnu
    aarch64-unknown-linux-gnu
    x86_64-apple-darwin
    aarch64-apple-darwin
    x86_64-pc-windows-msvc
  )
fi

cd "$(dirname "$0")/.."
dist="$PWD/build/dist"
mkdir -p "$dist"

sha256() {
  if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi
}

for target in "${targets[@]}"; do
  case "$target" in
    x86_64-unknown-linux-gnu)  goos=linux   goarch=amd64 ;;
    aarch64-unknown-linux-gnu) goos=linux   goarch=arm64 ;;
    x86_64-apple-darwin)       goos=darwin  goarch=amd64 ;;
    aarch64-apple-darwin)      goos=darwin  goarch=arm64 ;;
    x86_64-pc-windows-msvc)    goos=windows goarch=amd64 ;;
    *) echo "unknown target: $target" >&2; exit 1 ;;
  esac

  name="hurl-$version-$target"
  stage="$PWD/build/stage/$name"
  rm -rf "$stage"
  mkdir -p "$stage/bin"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath -ldflags="-s -w" -o "$stage/bin/" ./cmd/hurl ./cmd/hurlfmt

  if [ "$goos" = windows ]; then
    archive="$name.zip"
    rm -f "$dist/$archive"
    (cd "$stage" && cp ../../../LICENSE ../../../NOTICE ../../../README.md . && mv bin/* . && rmdir bin && zip -q -X "$dist/$archive" ./*)
  else
    archive="$name.tar.gz"
    cp LICENSE NOTICE README.md "$stage/"
    tar -czf "$dist/$archive" -C "$PWD/build/stage" "$name"
  fi
  (cd "$dist" && sha256 "$archive" > "$archive.sha256")
  echo "$dist/$archive"
done
