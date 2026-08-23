# sb-relay

一个使用 Go 的轻量烧饼论坛（sb.sb）通知转发器。通知通过
[Shoutrrr v0.8.0](https://containrrr.dev/shoutrrr/v0.8/services/overview/)
发送，可使用其全部稳定渠道以及通用 Webhook。

> 本分支是 Shoutrrr 预览版，镜像标签为
> `ghcr.io/krabdo/sb-relay-shoutrrr:shoutrrr-preview`

## 功能

- 默认每 60 秒检查一次通知，可通过 `POLL_INTERVAL` 修改。
- 首次启动只发送当前最新的一条，其余当前通知作为去重基线。
- 按通知 ID 去重，跨页补抓突发通知，并按从旧到新的顺序发送。
- 多个目标并发发送；至少一个目标成功即将通知记为已发送。全部失败才在下轮重试。
- Telegram 使用 HTML 富文本、关闭链接预览，并在正文内提供“查看原帖”链接。
- 其他渠道使用纯文本，并通过 Shoutrrr `title` 参数设置标题。

## 配置通知渠道

必须在下列两个变量中选择一个，不能同时设置：

- `SHOUTRRR_URLS`：一个或多个以空白分隔的 Shoutrrr 服务 URL。
- `SHOUTRRR_URLS_FILE`：文件路径，文件中每行一个服务 URL，忽略空行。

Telegram 示例：

```dotenv
SHOUTRRR_URLS='telegram://bot-id:bot-secret@telegram?chats=chat-id'
```

多个目标示例：

```dotenv
SHOUTRRR_URLS='telegram://bot-id:bot-secret@telegram?chats=chat-id discord://token@webhook-id'
```

URL 的完整写法请查阅
[Shoutrrr 服务列表](https://containrrr.dev/shoutrrr/v0.8/services/overview/)。
程序会强制 Telegram 目标使用 `parseMode=HTML` 和 `preview=No`，即使 URL 中指定了其他值。

服务 URL 通常包含令牌、密码或 Webhook 密钥：

- 在 Railway/Compose 中给整个值加引号。
- 使用平台 Secret 或只读挂载文件注入，不要提交到仓库。
- 程序不会在日志中输出服务 URL；投递错误仅显示目标序号、scheme 和脱敏原因。

旧版的 `TELEGRAM_BOT_TOKEN` 和 `TELEGRAM_CHAT_ID` 已删除，单独配置它们无法启动。

## 取得论坛 Cookie

1. 桌面浏览器登录 [烧饼论坛](https://sb.sb/)，打开自己的“通知”页面。
2. 按 `F12`，打开开发者工具的 **Network/网络** 面板并刷新页面。
3. 选择类型为 `Document` 的请求，在 **Request Headers/请求标头** 中复制
   `Cookie` 的完整值，但不要复制 `Cookie:` 前缀。

示例仅展示格式，不是真实凭据：

```text
__Host-bbs_csrf=replace-me; __Host-bbs_session=replace-me
```

Cookie 失效后更新 `SB_COOKIE` 并重启容器。程序只对论坛执行 GET，不会清空或标记通知。

## Railway 部署

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/sb-relay-shoutrrr?referralCode=pjiwS3&utm_medium=integration&utm_source=template&utm_campaign=generic)

1. 在 Railway 创建项目，选择 **Docker Image**。
2. 预览阶段填入 `ghcr.io/krabdo/sb-relay-shoutrrr:shoutrrr-preview`。
3. 在 **Variables / Raw Editor** 中配置：

```dotenv
SB_USER_ID=<你的数字 UID>
SB_COOKIE='<你的完整 Cookie>'
SHOUTRRR_URLS='telegram://bot-id:bot-secret@telegram?chats=chat-id'

POLL_INTERVAL=60s
STATE_FILE=/data/state.json
```

4. （可选）为 `/data` 添加持久卷，否则容器重建后会重新建立基线并再次发送最新一条。
5. 部署并在日志中确认 `sb-relay started`。

## Docker Compose 部署

```bash
git clone --branch codex/shoutrrr https://github.com/krabdo/sb-relay.git
cd sb-relay
cp .env.example .env
# 编辑 .env，替换全部示例值
docker compose pull
docker compose up -d
docker compose logs -f sb-relay
```

若使用文件 Secret，可不设置 `SHOUTRRR_URLS`，改为把文件只读挂载到容器并设置：

```dotenv
SHOUTRRR_URLS_FILE=/run/secrets/shoutrrr_urls
```

文件中每行一个 URL。两种来源同时出现、文件为空、URL 重复、scheme 未知或配置无效时，
程序会在启动阶段退出。

## 配置参考

| 环境变量 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `SB_USER_ID` | 是 | — | 论坛中的数字用户 ID |
| `SB_COOKIE` | 是 | — | 完整 Cookie 值，不含 `Cookie:` 前缀 |
| `SHOUTRRR_URLS` | 二选一 | — | 空白分隔的一个或多个服务 URL |
| `SHOUTRRR_URLS_FILE` | 二选一 | — | 每行一个服务 URL 的 Secret 文件 |
| `POLL_INTERVAL` | 否 | `60s` | Go duration 格式，最短 `10s` |
| `STATE_FILE` | 否 | `/data/state.json` | v1 `seen` 状态文件路径 |

现有 `state.json` 不需要迁移或清空。

## 投递语义

每个 URL 对应一个独立 Shoutrrr sender，所有目标并发投递。至少一个目标成功后，
通知 ID 会原子写入状态文件，失败目标不会单独补发；这样可以避免已成功渠道重复收到同一通知。
如果全部目标失败，状态不会更新，程序会在下一轮重试该通知，并停止当前批次。

所有消息按 UTF-8 字节安全截断到 3900 字节，不会切断中文字符或 Telegram HTML 实体。
Shoutrrr 当前不提供可配置的 Telegram Inline Keyboard，因此“查看原帖”使用正文链接。

## 排障

- `forum authentication failed`：Cookie 已过期、复制不完整或不属于 `SB_USER_ID`。
- `forum notification page structure changed`：论坛 HTML 结构变化；状态不会被覆盖。
- `invalid Shoutrrr destination #...`：检查对应序号的 URL scheme、凭据格式和必填查询参数。
- `all ... Shoutrrr destinations failed`：所有目标均投递失败；查看各目标序号和脱敏原因。
- `load persistent state` / `persist state`：检查 `/data` 权限与磁盘空间。程序会停止以防重放。

项目许可证：[Apache-2.0](LICENSE)。Shoutrrr 及其 MIT 许可证归属见
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
