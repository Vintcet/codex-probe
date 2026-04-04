# codex-probe

[English](README.md) · [中文](README_ZH.md)

---

**Codex credential management & diagnostic CLI tool** — login, check quota, test models, export CSV. Works on Windows / Linux / macOS.

---

## Install

**Pre-built binary (recommended)**

Download the binary for your platform from [Releases](../../releases):

| Platform | File |
|---|---|
| Linux x86-64 | `codex-probe-linux-amd64` |
| Linux ARM64 | `codex-probe-linux-arm64` |
| macOS Intel | `codex-probe-darwin-amd64` |
| macOS Apple Silicon | `codex-probe-darwin-arm64` |
| Windows x86-64 | `codex-probe-windows-amd64.exe` |

**Build from source**

```bash
git clone https://github.com/yourname/codex-probe
cd codex-probe
go build -o codex-probe ./cmd/codex-probe/
```

---

## Quick Start

```bash
# 1. Login and save credential
codex-probe --login ./tokens/

# 2. Check remaining quota
codex-probe --status ./tokens/my.json

# 3. Test all model endpoints
codex-probe --apitest ./tokens/

# 4. Quota + apitest, export to CSV
codex-probe --status --apitest --output result.csv ./tokens/
```

---

## Options

| Flag | Description |
|---|---|
| `--login` | OAuth PKCE flow, listen on `:1455`, write credential JSON |
| `-o <path>` | Login: explicit output file or directory |
| `--status` | Fetch usage quota (5h window + weekly window) |
| `--apitest` | Send a minimal request to each model, report availability (`--test` is an alias) |
| `--output <path.csv>` | Write results to CSV (requires `--status` or `--apitest`) |
| `--proxy <url>` | Proxy URL (`http://…` or `socks5://…`). Pass `""` to force direct |
| `--help` | Show help |

**Last positional argument (required):**

| | Description |
|---|---|
| `<file>` | A single credential JSON file |
| `<dir>` | Directory — processes all `*.json` files inside |

**Credential JSON format:**

```json
{
  "access_token": "eyJ...",
  "refresh_token": "...",
  "account_id": "user-...",
  "email": "you@example.com"
}
```

---

## Proxy Detection Order

1. `--proxy <url>` flag
2. `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` env vars
3. macOS — `scutil --proxy`
4. Windows — `HKCU\...\Internet Settings` registry
5. Direct connection

---

## CSV Columns

**`--status`**
```
file, account_id, email, plan_type,
5h_used_pct, 5h_reset_at,
weekly_used_pct, weekly_reset_at,
upstream_status, error
```

**`--apitest`** (one row per token; `available` is true if at least one of **3 randomly sampled** models succeeds)

```
file, account_id, sample_models, available
```

`sample_models` lists the sampled model names, separated by `;`.

---

## Region check

Geo / region detection runs only when you actually run `--login`, `--status`, or `--apitest`. Showing `--help` does not trigger it.

---

## How It Works

`codex-probe` replicates the [Codex CLI](https://github.com/openai/codex) OAuth PKCE flow:

1. Generates random `state` + PKCE `verifier / S256 challenge`
2. Opens `https://auth.openai.com/oauth/authorize` with Codex CLI's `client_id`
3. Listens on `localhost:1455/auth/callback` for the browser redirect
4. Exchanges `code + verifier` → `access_token + refresh_token`
5. Decodes JWT to extract `account_id` and `email`

Expired tokens are refreshed automatically on 401/403.

---

## License

MIT
