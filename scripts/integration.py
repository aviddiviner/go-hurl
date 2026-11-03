#!/usr/bin/env python3
"""Runs the upstream Hurl integration scripts against go-hurl and tallies results.

Unlike upstream's integration.py, this keeps going after a failure and honours a
skip list (scripts/integration_skip.txt). Expects the go-hurl binaries first on
PATH and the test servers started (bin/test/test_prerequisites.sh, or
scripts/start_servers.sh without the proxy).

Header order is a known difference (net/http doesn't keep it), so runs of `> `/`< `
header lines and JSON `headers` arrays are sorted by name on both the expected and
actual side before comparing. Missing or extra headers still fail.

    PATH=$PWD/build:$PATH python3 scripts/integration.py [glob ...]
"""

import glob
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
HURL_DIR = os.path.join(ROOT, "integration", "hurl")
DEFAULT_GLOBS = [
    "tests_ok/**/*.sh",
    "tests_ok_not_linted/*.sh",
    "tests_failed/**/*.sh",
    "tests_failed_not_linted/*.sh",
    "tests_error_parser/*.sh",
    "tests_ssl/*.sh",
]


def load_skips() -> dict[str, str]:
    skips = {}
    with open(os.path.join(ROOT, "scripts", "integration_skip.txt")) as f:
        for line in f:
            line = line.split("#", 1)
            path, reason = line[0].strip(), line[1].strip() if len(line) > 1 else ""
            if path:
                skips[path] = reason
    return skips


def main() -> int:
    globs = sys.argv[1:] or DEFAULT_GLOBS
    os.chdir(HURL_DIR)
    scripts = sorted({f for g in globs for f in glob.glob(g, recursive=True)})
    skips = load_skips()
    passed, failed, skipped = [], [], []
    for script in scripts:
        if script in skips:
            skipped.append(script)
            continue
        tty = "True" if not script.startswith("tests_pty/") else "False"
        code = (
            "import sys; sys.path[:0] = ['..', %r]; import integration_normalize; "
            "integration_normalize.install(); import test_script; test_script.test(%r, %s)"
        ) % (os.path.join(ROOT, "scripts"), script, tty)
        result = subprocess.run([sys.executable, "-c", code], capture_output=True, text=True)
        if result.returncode == 0:
            passed.append(script)
        else:
            failed.append(script)
            print(f"FAIL {script}\n{result.stdout}{result.stderr}")
    print(f"\npassed: {len(passed)}, failed: {len(failed)}, skipped: {len(skipped)}")
    for f in failed:
        print(f"  FAIL {f}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
