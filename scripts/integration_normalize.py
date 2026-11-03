"""Normalises header order for scripts/integration.py (see its docstring).

install() patches the upstream test_script module in place: expected files read
through its `open` and actual output from its terminals both go through
normalize(), which sorts runs of verbose header lines and JSON `headers` arrays
(one-line, or pretty-printed by jq) by header name.
"""

import builtins
import io
import re

import term
import test_script

VERBOSE_HEADER = re.compile(r"^([<>]) (?:\x1b\[[0-9;]*m)?([A-Za-z0-9!#$%&'*+.^_`|~-]+)(?:\x1b\[[0-9;]*m)?: ")
JSON_HEADERS = re.compile(r'"headers":\[((?:\{"name":"(?:[^"\\]|\\.)*","value":"(?:[^"\\]|\\.)*"\},?)*)\]')
JSON_HEADER = re.compile(r'\{"name":"((?:[^"\\]|\\.)*)","value":"(?:[^"\\]|\\.)*"\}')
PRETTY_HEADERS = re.compile(
    r'("headers": \[\n)((?: *\{\n *"name": "(?:[^"\\]|\\.)*",\n *"value": "(?:[^"\\]|\\.)*"\n *\},?\n)+)( *\])'
)
PRETTY_HEADER = re.compile(r' *\{\n *"name": "((?:[^"\\]|\\.)*)",\n *"value": "(?:[^"\\]|\\.)*"\n *\}')


def _sort_pretty_headers(m: re.Match) -> str:
    items = sorted(PRETTY_HEADER.finditer(m.group(2)), key=lambda h: h.group(1).lower())
    return m.group(1) + ",\n".join(h.group(0) for h in items) + "\n" + m.group(3)


def _sort_json_headers(m: re.Match) -> str:
    items = [h.group(0) for h in JSON_HEADER.finditer(m.group(1))]
    items.sort(key=lambda h: JSON_HEADER.match(h).group(1).lower())
    return '"headers":[' + ",".join(items) + "]"


def normalize(text: str) -> str:
    out, run = [], []
    for line in text.split("\n"):
        m = VERBOSE_HEADER.match(line)
        if m and (not run or run[0][0] == line[0]):
            run.append(line)
            continue
        out.extend(sorted(run, key=lambda h: VERBOSE_HEADER.match(h).group(2).lower()))
        run = [line] if m else []
        if not m:
            out.append(line)
    out.extend(sorted(run, key=lambda h: VERBOSE_HEADER.match(h).group(2).lower()))
    text = JSON_HEADERS.sub(_sort_json_headers, "\n".join(out))
    return PRETTY_HEADERS.sub(_sort_pretty_headers, text)


def _normalize_bytes(b):
    if not isinstance(b, bytes):
        return b
    return normalize(b.decode("utf-8", "surrogateescape")).encode("utf-8", "surrogateescape")


def _open(file, mode="r", *args, **kwargs):
    with builtins.open(file, "rb") as f:
        data = _normalize_bytes(f.read())
    if "b" in mode:
        return io.BytesIO(data)
    return io.TextIOWrapper(io.BytesIO(data), encoding=kwargs.get("encoding"), newline=kwargs.get("newline"))


def _wrap_run(run):
    def wrapped(self, cmd):
        result = run(self, cmd)
        result.stdout = _normalize_bytes(result.stdout)
        result.stderr = _normalize_bytes(result.stderr)
        return result

    return wrapped


def install():
    test_script.open = _open
    term.Term.run = _wrap_run(term.Term.run)
    term.PseudoTerm.run = _wrap_run(term.PseudoTerm.run)
