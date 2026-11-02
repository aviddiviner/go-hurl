#!/bin/bash
set -Eeuo pipefail

set +eo pipefail
hurl --version | grep Features | grep -q NTLM
if [ $? -eq 1 ]; then
  exit 255
fi
set -Eeuo pipefail

hurl --ntlm -u ":" tests_ok/ntlm/ntlm.hurl
