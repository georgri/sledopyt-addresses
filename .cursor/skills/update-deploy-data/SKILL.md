---
name: update-deploy-data
description: Update deploy data artifacts (KLADR BASE.7z + GAR overlay) for sledopyt-addresses locally and on VDS.
---

# Update Deploy Data

## Scope

This skill is specific to `sledopyt-addresses` and data refresh for deploy.

Targets:

- KLADR source (`BASE.7z`)
- GAR overlay (`gar_overlay.json`) for major-city regions (`77,78` by default)

Hosts:

- VDS deploy host: `root@88.210.9.155`

## Local data refresh

Run from repository root:

```bash
go run ./cmd/gar-fetch -regions 77,78 -out ./data/gar_overlay.json
```

`BASE.7z` is downloaded from official FIAS metadata endpoint by deploy updater script, so locally you may keep the existing file or download manually.

## VDS one-off data refresh

Sync latest repo first, then run updater script:

```bash
rsync -az --delete --exclude '.git/' --exclude 'data/' --exclude '.env' --exclude '._*' /Users/georgiiriskov/go/sledopyt-addresses/ root@88.210.9.155:/root/sledopyt-addresses/
ssh root@88.210.9.155 "cd ~/sledopyt-addresses && ./deploy/scripts/update-kladr-gar-data.sh"
```

If VDS cannot access FIAS endpoints directly, set relay in `/root/sledopyt-addresses/.env`:

```dotenv
DOWNLOAD_RELAY_HOST=root@185.231.154.188
DOWNLOAD_RELAY_SSH_KEY=/root/.ssh/id_rsa_georgri_github
RELAY_SOCKS_PORT=1080
```

Updater will open SSH SOCKS tunnel and fetch KLADR/GAR via relay host.

## Enable nightly refresh at 05:00 on VDS

```bash
ssh root@88.210.9.155 "cp ~/sledopyt-addresses/deploy/systemd/sledopyt-data-update.service /etc/systemd/system/ && cp ~/sledopyt-addresses/deploy/systemd/sledopyt-data-update.timer /etc/systemd/system/ && systemctl daemon-reload && systemctl enable --now sledopyt-data-update.timer && systemctl list-timers | grep sledopyt-data-update"
```

## Runtime requirements

- Keep `GAR_OVERLAY_REGIONS=77,78` in `.env` unless a different region set is needed.
- Updater uses ranged GAR download + streaming parse with `GOMEMLIMIT=1GiB`, plus low-priority I/O (`Nice=10`, best-effort I/O scheduling) to fit 8 GB RAM while bot stays online.
