#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Builds the site and starts the server, which serves both the site and the API: http://localhost:8080
# Settings come from .env (if present), see .env.example.
set -eu

cd "$(dirname "$0")"

if [ -f .env ]; then
	set -a
	. ./.env
	set +a
fi
: "${ADDR:=:8080}"
export ADDR

echo "==> Building the site"
if [ ! -d web/node_modules ]; then
	(cd web && npm install)
fi
log=$(mktemp)
if ! (cd web && npm run build) >"$log" 2>&1; then
	cat "$log"
	rm -f "$log"
	echo "Site build failed, see the log above." >&2
	exit 1
fi
rm -f "$log"

echo "==> Building the server"
(cd server && go build -o bin/server ./cmd/server)

port="${ADDR##*:}"
echo
echo "Site:              http://localhost:$port"
ip=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") print $(i + 1)}')
if [ -n "$ip" ]; then
	echo "From a phone (LAN): http://$ip:$port"
fi
echo "Stop: Ctrl+C"
echo

cd server
STATIC_DIR=../web/build exec ./bin/server
