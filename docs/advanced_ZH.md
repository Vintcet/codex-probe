# 进阶使用

这里主要放首页不展开的使用细节。

## 工作原理

`codex-probe` 走的是 Codex CLI 的 OAuth PKCE 授权流程：

1. 生成随机 `state` 和 PKCE `verifier` / `S256 challenge`
2. 打开 `https://auth.openai.com/oauth/authorize`
3. 监听 `localhost:1455/auth/callback`
4. 用 `code + verifier` 换取 `access_token`、`refresh_token`、`id_token`
5. 解码 JWT，提取 `account_id` 和 `email`

## 续期机制

`--renew` 是按“每个 token 文件单独判断”的方式执行的。

| 配置项 | 说明 |
|---|---|
| `renew_before_expiry_days` | token 距离过期小于等于这个天数时，会触发续期 |
| `-f` | 不看过期时间，直接强制续期 |
| 缺少 `id_token` | 视为可以续期，尝试刷新 |
| `expired` 缺失或格式不合法 | 视为可以续期，尝试刷新 |

详细规则：

1. 只要命中任一续期条件，就会刷新该 token。
2. 如果 token 还新鲜，且没有传 `-f`，就会跳过。
3. 每次刷新内部最多重试 3 次。
4. `--status` 遇到 `401` 或 `403` 时，也可能触发自动续期。
5. 续期成功后，如果文件位于 `sync_dir` 内，CLI 会继续问你要不要马上执行 `--sync`。

常见变体：

```bash
# 强制续期
codex-probe --renew -f ./tokens/me.json

# 指定配置文件再同步
codex-probe --config ./config.json --sync

# 导出 CSV
codex-probe --status --apitest --output result.csv ./tokens/
```

## 代理检测机制

不传 `--proxy` 时，`codex-probe` 会按下面顺序找代理：

1. `HTTPS_PROXY`、`HTTP_PROXY`、`ALL_PROXY`
2. macOS `scutil --proxy`
3. Windows Internet Settings 注册表
4. 直连

## CSV 格式

`--status` 导出列：

| 列名 | 说明 |
|---|---|
| `file` | 凭证文件路径 |
| `account_id` | 账号 ID |
| `email` | 邮箱 |
| `plan_type` | 套餐类型 |
| `5h_used_pct` | 5小时窗口已用百分比 |
| `5h_reset_at` | 5小时窗口重置时间 |
| `weekly_used_pct` | 每周窗口已用百分比 |
| `weekly_reset_at` | 每周窗口重置时间 |
| `upstream_status` | HTTP 状态码 |
| `error` | 错误信息 |

`--apitest` 导出列：

| 列名 | 说明 |
|---|---|
| `file` | 凭证文件路径 |
| `account_id` | 账号 ID |
| `sample_models` | 随机抽样的 3 个模型名，使用 `;` 分隔 |
| `available` | 至少有 1 个模型请求成功时为 `true` |
