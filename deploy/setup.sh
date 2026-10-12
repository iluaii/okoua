#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# One-time setup of the server laptop (Void Linux). Run as root on the laptop, from a copy
# of this repository's deploy/ directory:
#
#	sudo ./setup.sh example.pp.ua youruser
#
# youruser is the account deploy.sh logs in as over SSH. Safe to run again.
set -eu

domain=${1:?usage: setup.sh <domain> <deploy-user>}
owner=${2:?usage: setup.sh <domain> <deploy-user>}
here=$(cd "$(dirname "$0")" && pwd)
[ "$(id -u)" = 0 ] || { echo "run as root" >&2; exit 1; }
id "$owner" >/dev/null

echo "==> Packages"
# xbps treats "already installed" as an error, so ask only for what is missing.
missing=
for p in caddy sqlite sudo; do
	xbps-query "$p" >/dev/null 2>&1 || missing="$missing $p"
done
if [ -n "$missing" ]; then
	# shellcheck disable=SC2086 # one word per package
	xbps-install -Sy $missing || {
		echo "Package install failed. If it says \"unresolvable shlib\", update the system first:" >&2
		echo "  sudo xbps-install -Su   (run it again if it updated xbps itself)" >&2
		exit 1
	}
fi

echo "==> Users"
id alert >/dev/null 2>&1 || useradd -r -M -d /opt/alert-concept/var -s /bin/false alert
id alertweb >/dev/null 2>&1 || useradd -r -M -d /var/lib/alert-caddy -s /bin/false alertweb

echo "==> Directories"
# The deploy user owns the code; only the service user can read the data and keys.
install -d -o "$owner" /opt/alert-concept /opt/alert-concept/bin /opt/alert-concept/web
install -d -o alert -g alert -m 700 /opt/alert-concept/var
install -d -o alertweb -g alertweb -m 700 /var/lib/alert-caddy
install -d /etc/alert-concept /var/log/alert-concept /var/log/alert-caddy

echo "==> Configuration"
sed "s/^DOMAIN {/$domain {/" "$here/Caddyfile" >/etc/alert-concept/Caddyfile
if [ ! -f /etc/alert-concept/env ]; then
	cat >/etc/alert-concept/env <<ENV
# Server settings (see .env.example in the repository). Secrets live only here.
ADDR=127.0.0.1:8080
STATIC_DIR=/opt/alert-concept/web
DATA_DIR=/opt/alert-concept/var
TRUST_PROXY=1
VAPID_SUBJECT=https://$domain
ALERTS_IN_UA_TOKEN=
OPENROUTER_API_KEY=
ENV
fi
chown root:alert /etc/alert-concept/env
chmod 640 /etc/alert-concept/env

echo "==> Ports 80 and 443 for Caddy"
# Let Caddy bind ports 80 and 443 without running as root.
# Void reads /etc/sysctl.d/*.conf at boot but doesn't create the directory.
install -d /etc/sysctl.d
echo 'net.ipv4.ip_unprivileged_port_start=80' >/etc/sysctl.d/alert-caddy.conf
sysctl -q -w net.ipv4.ip_unprivileged_port_start=80

echo "==> Backup helper and sudo rule"
install -D -m 755 "$here/backup-local" /usr/local/sbin/alert-concept-backup
# deploy.sh and backup.sh need exactly these two commands as root, nothing else.
install -d -m 750 /etc/sudoers.d
grep -Eq '^[@#]includedir /etc/sudoers.d' /etc/sudoers ||
	echo "Warning: /etc/sudoers doesn't read /etc/sudoers.d; add the line  @includedir /etc/sudoers.d  with visudo" >&2
# Check the file before putting it in place: a broken file in sudoers.d breaks sudo entirely.
tmp=$(mktemp)
cat >"$tmp" <<SUDO
$owner ALL=(root) NOPASSWD: /usr/bin/sv restart alert-concept, /usr/sbin/sv restart alert-concept, /usr/local/sbin/alert-concept-backup
SUDO
if ! visudo -c -f "$tmp"; then
	rm -f "$tmp"
	echo "The sudo rule above is invalid; nothing was changed in /etc/sudoers.d." >&2
	exit 1
fi
# sudo applies the last matching rule and reads sudoers.d in name order, so the file must
# come after e.g. "wheel" (everything, with a password), or it would be overridden.
rm -f /etc/sudoers.d/alert-concept
install -m 440 "$tmp" /etc/sudoers.d/zz-alert-concept
rm -f "$tmp"

echo "==> Services"
for s in alert-concept alert-caddy; do
	rm -rf "/etc/sv/$s"
	cp -r "$here/sv/$s" /etc/sv/
done
# The server service waits until deploy.sh has put a build in place.
for s in alert-concept alert-caddy; do
	[ -e "/var/service/$s" ] || ln -s "/etc/sv/$s" /var/service/
done

echo
echo "Done. Next:"
echo "  1. Put the keys into /etc/alert-concept/env (OPENROUTER_API_KEY, later ALERTS_IN_UA_TOKEN)."
echo "  2. From your computer: deploy/deploy.sh $owner@<laptop address>"
