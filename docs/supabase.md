# Supabase Setup

This guide covers the cloud-side and local-side configuration required for `--sync`.

## Plan

Supabase Free Plan is enough for `codex-probe` token sync.

## Console Setup

1. Create a Supabase project.
2. Open SQL Editor.
3. Run [supabase.sql](../supabase.sql).
4. Copy the project URL and publishable key from the API settings page.

The current table shape is:

```sql
create table public.tokens (
  id text not null,
  email text null,
  token_data text not null,
  update_time timestamp with time zone null,
  constraint token_pkey primary key (id)
);
```

`id` uses local `account_id`. `email` is uploaded alongside the encrypted payload. `token_data` stores AES-GCM encrypted JSON generated on the client side.

## Local Config

Start from [config.example.json](../config.example.json):

```json
{
  "renew_before_expiry_days": 3,
  "sync_url": "https://<project>.supabase.co",
  "sync_api_key": "<publishable-key>",
  "sync_aes_gcm_key": "<64-char-hex>",
  "sync_dir": "./tokens"
}
```

Field reference:

| Field | Description |
|---|---|
| `sync_url` | Supabase project URL |
| `sync_api_key` | Supabase publishable key |
| `sync_aes_gcm_key` | 64-char hex key for AES-256-GCM |
| `sync_dir` | Local directory scanned by `--sync` |

Generate `sync_aes_gcm_key` with:

```bash
openssl rand -hex 32
```

## Sync Notes

`--sync` merges local and remote records by `account_id` and `update_time`.

- local only: keep local and upload it
- remote only: restore it locally as `<email>.json`
- both present, remote newer: overwrite local content but keep the existing local filename
- both present, local newer or equal: keep local content and upload it

If `sync_dir` does not exist yet, `--sync` treats it as an empty local store and creates directories when it restores remote tokens.

## Lost-Key Recovery

If `sync_aes_gcm_key` is lost, old cloud records cannot be decrypted anymore.

Recovery path:

1. Keep any valid local token files you still have.
2. Reset the remote `tokens` table.
3. Generate a new AES key.
4. Update `config.json`.
5. Run `--sync` again to upload from local files.
