Set-StrictMode -Version latest
$ErrorActionPreference = 'Stop'

$ErrorActionPreference = 'Continue'
hurl --version | grep Features | grep -q NTLM
if ($LASTEXITCODE -eq 1) {
  exit 255
}
$ErrorActionPreference = 'Stop'

hurl --ntlm -u ":" tests_ok/ntlm/ntlm.hurl
