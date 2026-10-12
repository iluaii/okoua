#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Builds the site and the server on this computer and ships them to the server laptop
# (set up once with setup.sh). Usage:
#
#	deploy/deploy.sh user@laptop
#
# The laptop needs neither Go nor Node. For a 32-bit laptop: GOARCH=386 deploy/deploy.sh …
set -eu

host=${1:?usage: deploy/deploy.sh user@laptop}
cd "$(dirname "$0")/.."
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT

echo "==> Building the site"
[ -d web/node_modules ] || (cd web && npm ci)
(cd web && npm run build) >"$out/web.log" 2>&1 || { cat "$out/web.log"; exit 1; }

echo "==> Building the server"
(cd server && CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH:-amd64}" go build -trimpath -ldflags='-s -w' -o "$out/server" ./cmd/server)

echo "==> Uploading"
# tar over ssh rather than rsync: nothing to install on either side. Remote commands go
# through sh explicitly, since the login shell there may be something else (fish).
tar -C web/build -cf - . | ssh "$host" "sh -c 'rm -rf /opt/alert-concept/web.new && mkdir /opt/alert-concept/web.new && tar -C /opt/alert-concept/web.new -xf -'"
ssh "$host" "sh -c 'cat >/opt/alert-concept/bin/server.new && chmod 755 /opt/alert-concept/bin/server.new'" <"$out/server"

echo "==> Switching to the new version"
ssh "$host" sh -s <<'REMOTE'
set -e
cd /opt/alert-concept
mv bin/server.new bin/server
rm -rf web.old
[ ! -d web ] || mv web web.old
mv web.new web
# The exact path in the sudoers rule; -n fails instead of asking for a password.
sudo -n /usr/sbin/sv restart alert-concept
REMOTE
echo "Done."
