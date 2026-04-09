# Supabase 配置

这里主要说明 `--sync` 需要的云端和本地配置。

## 套餐

Supabase 用 Free Plan 就够了，跑这个工具不需要额外上付费套餐。

## 控制台配置

1. 创建一个 Supabase 项目。
2. 打开 SQL Editor。
3. 执行 [supabase.sql](../supabase.sql)。
4. 到 API 设置页复制项目 URL 和 publishable key。

当前表结构：

```sql
create table public.tokens (
  id text not null,
  email text null,
  token_data text not null,
  update_time timestamp with time zone null,
  constraint token_pkey primary key (id)
);
```

其中 `id` 使用本地 `account_id`，`email` 会一并上传，`token_data` 保存的是客户端 AES-GCM 加密后的 JSON。

## 本地配置

建议从 [config.example.json](../config.example.json) 开始：

```json
{
  "renew_before_expiry_days": 3,
  "sync_url": "https://<project>.supabase.co",
  "sync_api_key": "<publishable-key>",
  "sync_aes_gcm_key": "<64-char-hex>",
  "sync_dir": "./tokens"
}
```

字段说明：

| 字段 | 说明 |
|---|---|
| `sync_url` | Supabase 项目地址 |
| `sync_api_key` | Supabase publishable key |
| `sync_aes_gcm_key` | AES-256-GCM 使用的 64 位十六进制密钥 |
| `sync_dir` | `--sync` 扫描和写回的本地目录 |

可以用下面命令生成 `sync_aes_gcm_key`：

```bash
openssl rand -hex 32
```

## 同步说明

`--sync` 会按 `account_id` 和 `update_time` 合并本地与云端记录。

- 本地有、云端无：保留本地并上传
- 云端有、本地无：恢复到本地，文件名使用 `<email>.json`
- 两边都有且云端更新：覆盖本地内容，但保持原文件名不变
- 两边都有且本地更新或时间相同：保留本地内容并上传

如果 `sync_dir` 还不存在，`--sync` 会把它当成空目录处理，并在恢复云端文件时自动创建目录。

## 密钥丢失后的恢复

如果丢了 `sync_aes_gcm_key`，旧的云端记录就没法再解密了。

恢复流程：

1. 先保留手头仍然有效的本地 token 文件。
2. 重置远端 `tokens` 表。
3. 生成新的 AES 密钥。
4. 更新 `config.json`。
5. 再执行一次 `--sync`，把本地 token 重新传回云端。
