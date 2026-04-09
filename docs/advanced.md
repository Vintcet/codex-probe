# Advanced Usage

This guide covers the operational details that do not need to stay on the project homepage.

## How It Works

`codex-probe` follows the Codex CLI OAuth PKCE flow:

1. Generate random `state` and PKCE `verifier` / `S256 challenge`
2. Open `https://auth.openai.com/oauth/authorize`
3. Listen on `localhost:1455/auth/callback`
4. Exchange `code + verifier` for `access_token`, `refresh_token`, and `id_token`
5. Decode JWT claims to extract `account_id` and `email`

## Renew Behavior

`--renew` evaluates each token file independently.

| Setting | Meaning |
|---|---|
| `renew_before_expiry_days` | Renew when a token will expire within this many days |
| `-f` | Force renew regardless of expiry time |
| missing `id_token` | Treat as renewable and try refresh |
| invalid or missing `expired` | Treat as renewable and try refresh |

Detailed rules:

1. A token is renewed when any renew condition matches.
2. A token is skipped when it is still fresh and `-f` is not set.
3. Each refresh request retries up to 3 times internally.
4. `--status` can also auto-refresh on `401` or `403`.
5. After a successful renew, the CLI can prompt for `--sync` when the refreshed file belongs to `sync_dir`.

Useful command variants:

```bash
# Force renew
codex-probe --renew -f ./tokens/me.json

# Use a custom config file
codex-probe --config ./config.json --sync

# Export CSV
codex-probe --status --apitest --output result.csv ./tokens/
```

## Proxy Detection

When `--proxy` is omitted, `codex-probe` checks proxy settings in this order:

1. `HTTPS_PROXY`, `HTTP_PROXY`, `ALL_PROXY`
2. macOS `scutil --proxy`
3. Windows Internet Settings registry
4. direct connection

## CSV Output

`--status` columns:

| Column | Description |
|---|---|
| `file` | Credential file path |
| `account_id` | Account ID |
| `email` | Email |
| `plan_type` | Plan type |
| `5h_used_pct` | 5-hour window usage percent |
| `5h_reset_at` | 5-hour reset time |
| `weekly_used_pct` | Weekly usage percent |
| `weekly_reset_at` | Weekly reset time |
| `upstream_status` | HTTP status code |
| `error` | Error message when present |

`--apitest` columns:

| Column | Description |
|---|---|
| `file` | Credential file path |
| `account_id` | Account ID |
| `sample_models` | 3 sampled model names separated by `;` |
| `available` | `true` when at least one sampled model succeeds |
