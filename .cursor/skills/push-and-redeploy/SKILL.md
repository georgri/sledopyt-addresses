---
name: push-and-redeploy
description: Push current project state to GitHub and redeploy to the production VDS for this repository. Use after code changes when the user asks to publish/deploy the latest version.
---

# Push And Redeploy

## Scope

This skill is specific to `sledopyt-addresses` and uses:

- Deploy host: `root@88.210.9.155`
- Auth/push host: `georgri@185.157.214.216`
- GitHub repo: `git@github.com:georgri/sledopyt-addresses.git`

## Workflow

1. Validate local code compiles.
2. Sync local workspace to VDS and restart service.
3. Sync local workspace to auth host clone and push to GitHub.
4. Report deployment status and commit hash.

## Commands

### 1) Local validation

Run from repository root:

```bash
go test ./... && go build ./cmd/sledopyt-addresses
```

### 2) Redeploy to VDS

```bash
rsync -az --delete --exclude '.git/' --exclude 'data/' --exclude '.env' --exclude '._*' /Users/georgiiriskov/go/sledopyt-addresses/ root@88.210.9.155:/root/sledopyt-addresses/
ssh root@88.210.9.155 "cd ~/sledopyt-addresses && /usr/bin/docker-compose down && /usr/bin/docker-compose up --build -d && sleep 45 && /usr/bin/docker-compose logs --tail=50"
```

### 3) Push current state to GitHub via auth host

```bash
COPYFILE_DISABLE=1 tar -czf /tmp/sledopyt-addresses-sync.tar.gz -C /Users/georgiiriskov/go sledopyt-addresses
scp /tmp/sledopyt-addresses-sync.tar.gz georgri@185.157.214.216:/tmp/
ssh georgri@185.157.214.216 "mkdir -p /tmp/sledopyt-addresses-src-sync && rm -rf /tmp/sledopyt-addresses-src-sync/* && tar -xzf /tmp/sledopyt-addresses-sync.tar.gz -C /tmp/sledopyt-addresses-src-sync --strip-components=1 && if [ ! -d /tmp/sledopyt-addresses-repo/.git ]; then git clone git@github.com:georgri/sledopyt-addresses.git /tmp/sledopyt-addresses-repo; fi && cd /tmp/sledopyt-addresses-repo && rsync -az --delete --exclude '.git/' /tmp/sledopyt-addresses-src-sync/ ./ && git add -A && if ! git diff --cached --quiet; then git commit -m \"sync: deploy latest local state\"; fi && git push origin main"
```

## Output Checklist

Always report:

- Whether local build/tests passed
- Whether VDS service restarted and loaded successfully
- Whether GitHub push succeeded
- Final commit SHA on `main`
