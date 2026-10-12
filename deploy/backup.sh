#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copies the database and the push keys from the server laptop to this computer.
# Losing vapid.json breaks every push subscription, so keep these copies. Usage:
#
#	deploy/backup.sh user@laptop [directory]   (default: backups/)
set -eu

host=${1:?usage: deploy/backup.sh user@laptop [directory]}
dir=${2:-backups}
mkdir -p "$dir"
file="$dir/alert-concept-$(date +%Y-%m-%d_%H%M).tar.gz"
ssh "$host" sudo /usr/local/sbin/alert-concept-backup >"$file.part"
mv "$file.part" "$file"
echo "Saved $file"
