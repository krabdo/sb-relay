# sb-relay

一个尽量轻量的烧饼论坛（sb.sb）通知转发器：定时读取用户通知页，并把新增通知发送到 Telegram。程序使用静态 Go 二进制和服务端 HTML，不需要 Chromium、WebDriver 或论坛密码。

## 功能

- 每 60 秒检查一次通知，可通过 `POLL_INTERVAL` 修改。
- 支持回复、提及及页面未来出现的其他通知类型。
- 首次启动只发送当前最新的一条，已有其余通知作为基线，不会刷屏。
- 按通知 ID 去重，突发通知会跨页补抓并按从旧到新的顺序发送。
- 状态原子写入 `/data/state.json`，容器重启不会重复发送。
- Cookie 失效、登录页跳转和页面结构变化会被明确报告。
- 镜像以非 root 用户运行，不包含 shell 或浏览器。

## 准备 Telegram

1. 在 Telegram 中联系 [@BotFather](https://t.me/BotFather)，创建机器人并取得 Bot Token。
2. 向机器人发送一条消息；如果目标是群组或频道，先把机器人加入目标并授予发送消息权限。
3. 通过 Telegram Bot API 的 `getUpdates` 响应取得目标 `chat.id`。频道 ID 和部分群组 ID 通常以 `-100` 开头。

## 取得论坛 Cookie

1. 在桌面浏览器登录 [烧饼论坛](https://sb.sb/)，打开自己的“通知”页面。
2. 打开浏览器开发者工具的 **Network/网络** 面板并刷新页面。
3. 选择通知页的文档请求，在 **Request Headers/请求标头** 中复制 `Cookie` 的完整值。
4. 只复制冒号后面的值，不要包含 `Cookie:` 字样。

Cookie 等同于论坛登录凭据。不要把它写入 Dockerfile、提交到 Git、粘贴到公开日志或发送给他人。Cookie 失效后，重新执行上述步骤并重启容器。

## Docker Compose 部署

复制示例配置：

```bash
cp .env.example .env
chmod 600 .env
```

编辑 `.env`：

```dotenv
SB_USER_ID=1234
SB_COOKIE='完整的 Cookie 请求头值'
TELEGRAM_BOT_TOKEN=123456789:机器人令牌
TELEGRAM_CHAT_ID=123456789

POLL_INTERVAL=60s
STATE_FILE=/data/state.json
```

Cookie 建议使用单引号，避免 `$`、`#` 等字符被 Compose 解释。启动服务：

```bash
docker compose pull
docker compose up -d
docker compose logs -f sb-relay
```

`compose.yaml` 使用命名卷 `sb-relay-data` 保存状态。不要在正常升级时删除此卷，否则下一次启动会重新执行“只发送最新一条”的首次启动逻辑。

也可以直接运行镜像：

```bash
docker run -d \
  --name sb-relay \
  --restart unless-stopped \
  --env-file .env \
  -v sb-relay-data:/data \
  ghcr.io/krabdo/sb-relay:0.1.0
```

支持 `linux/amd64` 和 `linux/arm64`。

## 配置

| 环境变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `SB_USER_ID` | 是 | — | 通知页 URL 中的数字用户 ID |
| `SB_COOKIE` | 是 | — | 完整 Cookie 值，不含 `Cookie:` 前缀 |
| `TELEGRAM_BOT_TOKEN` | 是 | — | Telegram Bot Token |
| `TELEGRAM_CHAT_ID` | 是 | — | 单个私聊、群组或频道 ID |
| `POLL_INTERVAL` | 否 | `60s` | Go duration 格式，最短 `10s` |
| `STATE_FILE` | 否 | `/data/state.json` | 去重状态文件路径 |

## 升级与排障

```bash
docker compose pull
docker compose up -d
```

- `forum authentication failed`：Cookie 已过期、复制不完整或不属于 `SB_USER_ID`，更新 `.env` 后重启。
- `forum notification page structure changed`：论坛页面结构发生变化；状态不会被覆盖，请升级程序或提交 issue。
- `Telegram API error`：检查 Bot Token、chat ID、机器人在群组/频道中的权限及 Telegram 返回的错误说明。
- `load persistent state` / `persist state`：检查命名卷权限与磁盘空间。程序会停止，避免无法去重时反复发送。
- 新通知很多时，程序最多向后检查 10 页；找不到已知边界会在日志中给出警告并发送已收集的通知。

## 本地开发

```bash
go test ./...
go vet ./...
CGO_ENABLED=0 go build -trimpath ./cmd/sb-relay
```

查看版本：

```bash
go run ./cmd/sb-relay --version
```

许可证：[Apache-2.0](LICENSE)
