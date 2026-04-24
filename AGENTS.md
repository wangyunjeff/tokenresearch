# AGENTS.md

## Local Toolchain Notes For This Machine

- Codex shell commands run in a non-interactive login shell.
- `~/.bashrc` returns early for non-interactive shells:
  `case $- in *i*) ;; *) return;; esac`
- Because of that early return, Conda and NVM setup in `~/.bashrc` do not run for Codex shell commands.
- Result: tools that exist in user-managed environments may be present on the machine but missing from the default `PATH` seen by Codex.

## Go

- Working Go binary on this machine:
  `/home/ike/.conda/envs/service_codex_runtime/go/bin/go`
- Verified version:
  `go version go1.26.1 linux/amd64`
- If `go` is reported as missing, use the absolute path above or prepend it to `PATH`.

Example:

```bash
PATH="/home/ike/.conda/envs/service_codex_runtime/go/bin:$PATH" \
  /home/ike/.conda/envs/service_codex_runtime/go/bin/go version
```

For backend builds that need module downloads, use the local proxy:

```bash
PATH="/home/ike/.conda/envs/service_codex_runtime/go/bin:$PATH" \
HTTP_PROXY="http://127.0.0.1:7890" \
HTTPS_PROXY="http://127.0.0.1:7890" \
ALL_PROXY="socks5://127.0.0.1:7890" \
make build-backend
```

## Node And pnpm

- Working Node binary on this machine:
  `/home/ike/.nvm/versions/node/v22.19.0/bin/node`
- `pnpm` is not installed as a standalone binary in the default `PATH`.
- Use Corepack from the same Node install:
  `/home/ike/.nvm/versions/node/v22.19.0/bin/corepack`

Examples:

```bash
PATH="/home/ike/.nvm/versions/node/v22.19.0/bin:$PATH" \
  corepack pnpm --version
```

```bash
PATH="/home/ike/.nvm/versions/node/v22.19.0/bin:$PATH" \
  corepack pnpm --dir frontend run build
```

## Quick Recovery Checklist

If a command says `go: not found` or `pnpm: not found`:

1. Check whether the tool is installed in a user-managed environment, not system-wide.
2. Prefer the known absolute paths in this file.
3. If Go needs network access, route it through `127.0.0.1:7890`.
4. Do not assume interactive-shell setup from `~/.bashrc` is active inside Codex shell commands.
