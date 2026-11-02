Set-StrictMode -Version latest
$ErrorActionPreference = 'Stop'

$ErrorActionPreference = 'Continue'
hurl --version | grep Features | grep -q NTLM
if ($LASTEXITCODE -eq 1) {
  exit 255
}
$ErrorActionPreference = 'Stop'

hurl --verbose tests_ok/ntlm/ntlm_option.hurl
