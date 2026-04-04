# codex-probe

[English](README.md) · [中文](README_ZH.md)

---

**Codex 凭证管理与接口诊断命令行工具** — 一键登录、查看用量、测试模型、导出 CSV。支持 Windows / Linux / macOS。

---

## 安装

**直接下载（推荐）**

从 [Releases](../../releases) 下载对应平台的二进制文件：

| 平台 | 文件名 |
|---|---|
| Linux x86-64 | `codex-probe-linux-amd64` |
| Linux ARM64 | `codex-probe-linux-arm64` |
| macOS Intel | `codex-probe-darwin-amd64` |
| macOS Apple Silicon | `codex-probe-darwin-arm64` |
| Windows x86-64 | `codex-probe-windows-amd64.exe` |

**从源码编译**

```bash
git clone https://github.com/yourname/codex-probe
cd codex-probe
go build -o codex-probe ./cmd/codex-probe/
```

---

## 快速上手

```bash
# 1. 登录并保存凭证
codex-probe --login ./tokens/

# 2. 查看剩余用量
codex-probe --status ./tokens/my.json

# 3. 测试所有模型接口
codex-probe --apitest ./tokens/

# 4. 用量 + apitest，同时导出 CSV
codex-probe --status --apitest --output result.csv ./tokens/
```

---

## 参数说明

| 参数 | 说明 |
|---|---|
| `--login` | OAuth PKCE 登录，监听 `:1455` 回调，写入凭证 JSON |
| `-o <path>` | 登录：显式指定输出文件或目录；填写时优先于位置参数，也可单独使用 |
| `--status` | 查询剩余用量（5小时窗口 + 每周窗口）|
| `--apitest` | 对每个模型发最小请求，终端展示可用性（`--test` 为别名） |
| `--output <path.csv>` | 将结果写入 CSV（需配合 `--status` 或 `--apitest`）|
| `--proxy <url>` | 指定代理（`http://…` 或 `socks5://…`）；传 `""` 强制直连 |
| `--help` | 显示帮助 |

**最后一个位置参数（必需）：**

| | 说明 |
|---|---|
| `<file>` | 单个凭证 JSON 文件 |
| `<dir>` | 目录 — 批量处理目录下所有 `*.json` |

**凭证 JSON 格式：**

```json
{
  "access_token": "eyJ...",
  "refresh_token": "...",
  "account_id": "user-...",
  "email": "you@example.com"
}
```

---

## 代理检测优先级

1. `--proxy <url>` 命令行参数
2. `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY` 环境变量
3. macOS — `scutil --proxy`（系统网络偏好设置）
4. Windows — `HKCU\...\Internet Settings` 注册表
5. 以上均未检测到则直连

---

## CSV 输出列

**`--status`**
```
file, account_id, email, plan_type,
5h_used_pct, 5h_reset_at,
weekly_used_pct, weekly_reset_at,
upstream_status, error
```

**`--apitest`**（每个 token 一行；从全部结果中**随机抽 3 个模型**，若其中**至少 1 个**可用则 `available` 为 true）

```
file, account_id, sample_models, available
```

`sample_models` 为被抽样的模型名，以 `;` 分隔。

---

## 区域检测

仅在会执行 `--login` / `--status` / `--apitest` 时才会做外网区域检测；仅打印帮助时不会发起检测。

---

## 实现原理

`codex-probe` 复刻了 [Codex CLI](https://github.com/openai/codex) 的 OAuth PKCE 授权流程：

1. 生成随机 `state` + PKCE `verifier / S256 challenge`
2. 使用 Codex CLI 的 `client_id` 打开 `https://auth.openai.com/oauth/authorize`
3. 监听 `localhost:1455/auth/callback` 等待浏览器回调
4. 用 `code + verifier` 换取 `access_token + refresh_token`
5. 解码 JWT 提取 `account_id` 和 `email`

遇到 401/403 时自动用 `refresh_token` 续期。

---

## License

MIT
