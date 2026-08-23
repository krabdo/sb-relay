# sb-relay

一个使用 Go 的轻量烧饼论坛（sb.sb）通知转发器。

## Features

- 每 60 秒检查一次通知，可通过 `POLL_INTERVAL` 修改。
- 首次启动会发送当前最新的一条通知。
- 按通知 ID 去重，突发通知会跨页补抓并按从旧到新的顺序发送。

## 部署

### 1. 准备 Telegram

1. 在 Telegram 中搜索 [@BotFather](https://t.me/BotFather)，创建机器人并取得 Bot Token。
2. 再搜索 [@userinfobot](https://t.me/userinfobot)，获取自己的 Telegram ID。

记得先与你的机器人开始聊天，以免收不到消息。

### 2. 取得 Cookie

1. 桌面浏览器登录 [烧饼论坛](https://sb.sb/)，打开自己的“通知”页面。
2. 按 `F12`，打开浏览器开发者工具的 **Network/网络** 面板并刷新页面。
3. 选择**类型**为 `Document` 的请求，在 **Request Headers/请求标头** 中复制 `Cookie` 的完整值。你应该会得到这样的内容：
```
__Host-bbs_csrf=***; __Host-bbs_session=***; bbs_recent_forums=***; bbs_viewed=***
```

### 3.1 在 Railway 上部署（⭐️推荐）

*创建一键部署的 Railway Template 需要买他的 5$/M 的套餐，没办法了，只能给手动部署的教程。*

Railway 是一个容器托管平台，提供 1$/月 的免费试用额度，这个服务常驻内存占用不到 10MB，可以放心在上面 7x24 小时运行。

1. 注册 [Railway](https://railway.com?referralCode=MKpszV)（Aff）
2. 进入 Project 页面，点击 **+ New** 新建项目。
3. 选择 **Docker Image**
4. 填入 `ghcr.io/krabdo/sb-relay:latest`，回车。
5. 点击 **Variable**，进入 **Raw Editor**，填写下面的内容：
```dotenv
SB_USER_ID=<你的数字UID>
SB_COOKIE='<你的Cookie>'
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=

POLL_INTERVAL=60s
STATE_FILE=/data/state.json
```
- 请参考下一节 **配置** 填写缺失参数。
6. 点击 **Deploy** 按钮。

正常情况下，你就能在 Telegram 里收到你的最后一条通知了。

如果你不想在容器重启时，会重复收到最后一条消息，请添加持久卷。

### 3.2 Docker Compose 部署

如果你有网络可以到连接烧饼论坛和 Telegram 的 VPS，可以使用 Docker Compose 部署。

```bash
cd /opt
git clone https://github.com/krabdo/sb-relay.git
cd sb-relay
nano .env
```

写入下面的内容：

```dotenv
SB_USER_ID=<你的数字UID>
SB_COOKIE='<你的Cookie>'
TELEGRAM_BOT_TOKEN=
TELEGRAM_CHAT_ID=

POLL_INTERVAL=60s
STATE_FILE=/data/state.json
```

```bash
docker compose pull
docker compose up -d
docker compose logs -f sb-relay
```

## 配置

| 环境变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `SB_USER_ID` | 是 | — | 论坛中的数字用户 ID |
| `SB_COOKIE` | 是 | — | 完整 Cookie 值，不含 `Cookie:` 前缀 |
| `TELEGRAM_BOT_TOKEN` | 是 | — | Telegram Bot Token |
| `TELEGRAM_CHAT_ID` | 是 | — | 单个私聊、群组或频道 ID |
| `POLL_INTERVAL` | 否 | `60s` | Go duration 格式，最短 `10s` |
| `STATE_FILE` | 否 | `/data/state.json` | 状态文件路径 |

## 排障

- `forum authentication failed`：Cookie 已过期、复制不完整或不属于 `SB_USER_ID`，更新 `.env` 后重启。
- `forum notification page structure changed`：论坛页面结构发生变化；状态不会被覆盖，请升级程序或提交 issue。
- `Telegram API error`：检查 Bot Token、chat ID、机器人在群组/频道中的权限及 Telegram 返回的错误说明。
- `load persistent state` / `persist state`：检查命名卷权限与磁盘空间。程序会停止，避免无法去重时反复发送。
- 新通知很多时，程序最多向后检查 10 页；找不到已知边界会在日志中给出警告并发送已收集的通知。

许可证：[Apache-2.0](LICENSE)
