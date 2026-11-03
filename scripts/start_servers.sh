#!/bin/bash
# Starts the upstream integration test servers in the background (no squid),
# using the project .venv. Stop them with: pkill -f 'integration/hurl.*server.py'
set -Eeuo pipefail
cd "$(dirname "$0")/../integration/hurl"
py=../../.venv/bin/python
mkdir -p build
$py server.py --host 127.0.0.1 --port 8000 > build/server.log 2>&1 &
$py server.py --host "::1" --port 8004 > build/server-ipv6.log 2>&1 &
$py tests_ssl/ssl_server.py 8001 tests_ssl/certs/server/cert.selfsigned.pem false > build/server-ssl-selfsigned.log 2>&1 &
$py tests_ssl/ssl_server.py 8002 tests_ssl/certs/server/cert.pem false > build/server-ssl-signedbyca.log 2>&1 &
$py tests_ssl/ssl_server.py 8003 tests_ssl/certs/server/cert.selfsigned.pem true > build/server-ssl-client-authent.log 2>&1 &
$py tests_unix_socket/unix_socket_server.py > build/server-unix-socket.log 2>&1 &
for port in 8000 8001 8002 8003; do
  for _ in $(seq 30); do nc -z 127.0.0.1 $port 2>/dev/null && break; sleep 0.5; done
done
echo "servers up"
