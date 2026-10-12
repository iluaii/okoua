# Deploying on a home laptop

The server runs on an old laptop at home (Void Linux, public static IP), behind Caddy,
which gets and renews the HTTPS certificate. The site and the server are built on your
computer and copied over, so the laptop needs neither Go nor Node.

```
internet → router :80/:443 → laptop: Caddy (alertweb) → 127.0.0.1:8080 server (alert)
```

| On the laptop | What |
|---|---|
| `/opt/alert-concept/bin/server`, `web/` | the build, owned by the deploy user |
| `/opt/alert-concept/var/` | database and push keys (`vapid.json`), only the `alert` user |
| `/etc/alert-concept/env` | settings and secrets (see `.env.example`) |
| `/etc/alert-concept/Caddyfile` | Caddy config with the domain |
| `/etc/sv/alert-concept`, `/etc/sv/alert-caddy` | runit services; logs in `/var/log/alert-*/current` |

## Once

1. **Domain.** Register a free `*.pp.ua` domain and point its A record to your public IP.
2. **Router.** Forward TCP ports 80 and 443 to the laptop's LAN address (give the laptop
   a fixed address in the router's DHCP settings). Port 80 is needed for certificates.
3. **Laptop.** Copy `deploy/` to it and run:
   ```sh
   sudo ./setup.sh example.pp.ua youruser
   ```
   Then put the keys into `/etc/alert-concept/env` (`OPENROUTER_API_KEY`, later
   `ALERTS_IN_UA_TOKEN`) and set up SSH login by key for `youruser`.
4. **Power.**
   - Don't sleep on a closed lid: `HandleLidSwitch=ignore` and
     `HandleLidSwitchExternalPower=ignore` in `/etc/elogind/logind.conf`.
   - In the BIOS, turn on starting when AC power comes back, if there is such an option.
   - Put the router on a power bank or a DC UPS: the laptop has its battery, the router doesn't.
5. **Monitoring.** A free check on uptimerobot.com for `https://example.pp.ua/api/alerts`
   tells you when the site is down.

## Every update

From the repository on your computer:

```sh
deploy/deploy.sh youruser@laptop    # builds, uploads, restarts the server
deploy/backup.sh youruser@laptop    # copies the database and push keys to backups/
```

Back up regularly: losing `vapid.json` breaks every push subscription.

## Useful

```sh
sudo sv status alert-concept alert-caddy
sudo tail -f /var/log/alert-concept/current
```

To read feedback and AI answer reports, unpack a backup and run on your computer:

```sh
tar -xzf backups/alert-concept-….tar.gz -C /tmp app.db
cd server && go run ./cmd/feedback -db /tmp/app.db
```
