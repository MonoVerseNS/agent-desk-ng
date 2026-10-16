# 嵌入客服挂件

挂件会在任意网页上添加一个聊天入口：只需加载一个脚本，页面上显示悬浮按钮，点击后在 iframe 中打开客服聊天。访客可以同时保留多个请求并随时切换。

[English](./embedding.md) | [Русский](./embedding.ru-RU.md)

## 快速开始

将 `https://support.example.com` 替换为你的部署地址，将 `YOUR_CHANNEL_ID` 替换为控制台中的渠道 ID。

```html
<script>
  window.AgentDeskConfig = {
    channelId: "YOUR_CHANNEL_ID",
  }
</script>
<script src="https://support.example.com/sdk/agent-desk-sdk.min.js" async></script>
```

两段标签放在 `</body>` 之前，且 `AgentDeskConfig` 必须在 SDK 脚本**之前**执行。挂件会自动挂载，按钮出现在页面角落。

标题、副标题、主题色、位置和宽度来自控制台的渠道设置，通常无需在代码中配置。

## 配置项

| 选项 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `channelId` | `string` | – | **必填**，挂件所属渠道。 |
| `baseUrl` | `string` | 脚本所在域名 | Agent Desk 服务地址。 |
| `apiBaseUrl` | `string` | `baseUrl` | API 与页面不同源时使用。 |
| `widgetBaseUrl` | `string` | 脚本所在域名 | 聊天页面的加载地址。 |
| `language` | `"zh-CN" \| "en-US"` | 服务端设置 | 界面语言，任何 `en*` 取值都会选择英文。 |
| `position` | `"left" \| "right"` | 渠道设置 | 按钮和窗口所在一侧。 |
| `themeColor` | `string` | 渠道设置 | 主题色，例如 `#0f6cbd`。 |
| `width` | `string` | `380px` | 桌面端窗口宽度。 |
| `title`、`subtitle` | `string` | 渠道设置 | 窗口标题文字。 |
| `externalId` | `string` | 浏览器本地 ID | 你自己系统中的稳定用户 ID。 |
| `signExternalId` | `(id) => {signature, issuedAt}` | – | 证明 `externalId` 真实有效，见[已识别访客](#已识别访客)。 |
| `externalName` | `string` | – | 显示名称，仅在首次创建访客时使用。 |
| `getUserToken` | `() => string \| Promise<string>` | – | 返回由你的系统签发的 JWT。 |

屏幕宽度不超过 480px 时，聊天窗口会全屏打开。

## JavaScript API

```js
AgentDeskWidget.open()       // Promise<void>，打开聊天
AgentDeskWidget.close()      // 最小化聊天
AgentDeskWidget.mount(cfg)   // 使用显式配置挂载
AgentDeskWidget.destroy()    // 移除按钮和聊天窗口
AgentDeskWidget.getChatUrl() // Promise<string>，用于独立页面或新标签页
```

从自己的按钮打开聊天：

```html
<button onclick="AgentDeskWidget.open()">联系客服</button>
```

## React 与 Next.js

只在客户端加载一次脚本。Next.js（App Router）示例：

```tsx
"use client"

import Script from "next/script"

export function SupportWidget({ channelId }: { channelId: string }) {
  return (
    <>
      <Script id="agent-desk-config" strategy="afterInteractive">
        {`window.AgentDeskConfig = ${JSON.stringify({ channelId })}`}
      </Script>
      <Script
        src="https://support.example.com/sdk/agent-desk-sdk.min.js"
        strategy="afterInteractive"
      />
    </>
  )
}
```

包含函数的配置（`signExternalId`、`getUserToken`）无法序列化，请手动挂载：

```tsx
useEffect(() => {
  const script = document.createElement("script")
  script.src = "https://support.example.com/sdk/agent-desk-sdk.min.js"
  script.onload = () =>
    window.AgentDeskWidget?.mount({
      channelId,
      externalId: user.id,
      signExternalId: (id) => fetch(`/api/support/sign?id=${id}`).then((r) => r.json()),
    })
  document.body.appendChild(script)
  return () => window.AgentDeskWidget?.destroy()
}, [channelId, user.id])
```

## 已识别访客

默认情况下访客是由浏览器本地 ID 标识的匿名游客。若要把聊天关联到你自己的用户，需要同时传入 `externalId` 和签名，防止他人冒充：

```
signature = hex(HMAC-SHA256(identitySecret, channelId + "\n" + externalId + "\n" + issuedAt))
```

`issuedAt` 为当前 Unix 时间（秒）。服务端只接受未超过 `identity.maxAgeMinutes` 的签名，因此每次打开聊天时挂件都会重新获取签名。

> [!IMPORTANT]
> 签名必须在**你的服务端**计算，身份密钥绝不能下发到浏览器。

Node.js 接口：

```js
import crypto from "node:crypto"

app.get("/api/support/sign", requireLogin, (req, res) => {
  const issuedAt = Math.floor(Date.now() / 1000)
  const payload = `${CHANNEL_ID}\n${req.user.id}\n${issuedAt}`
  const signature = crypto
    .createHmac("sha256", process.env.AGENT_DESK_IDENTITY_SECRET)
    .update(payload)
    .digest("hex")
  res.json({ signature, issuedAt })
})
```

Go：

```go
func sign(secret, channelID, externalID string) (string, int64) {
    issuedAt := time.Now().Unix()
    mac := hmac.New(sha256.New, []byte(secret))
    fmt.Fprintf(mac, "%s\n%s\n%d", channelID, externalID, issuedAt)
    return hex.EncodeToString(mac.Sum(nil)), issuedAt
}
```

注意：接口签名的 `externalId` 应取自已登录用户，而不是查询参数。签名缺失或无效不会导致挂件失效，访客只会保持游客身份。

## 内容安全策略（CSP）与跨域

如果站点设置了 CSP，需要放行 Agent Desk 的域名：

```
script-src https://support.example.com;
frame-src  https://support.example.com;
connect-src https://support.example.com;
```

挂件与 API 不同源时，请把你的站点加入部署的允许来源，否则配置请求会失败。

## 常见问题

| 现象 | 原因与解决 |
| --- | --- |
| 没有出现按钮 | 缺少 `channelId`，或 `AgentDeskConfig` 定义在 SDK 脚本之后。查看控制台是否有 `channelId is required`。 |
| 按钮出现但聊天空白 | iframe 被浏览器拦截。检查 CSP 的 `frame-src`，以及代理是否为 Agent Desk 域名设置了 `X-Frame-Options`。 |
| 访客始终显示为游客 | 签名缺失、已过期，或使用了错误的密钥或拼接顺序。请对照 `identity.maxAgeMinutes` 检查服务器时钟偏差。 |
| 语言不对 | 显式设置 `language`，否则使用服务端默认语言。 |
| 单页应用中挂件重复 | 组件卸载时调用 `AgentDeskWidget.destroy()`。 |
| 聊天显示“出了点问题” | 点击**重新加载聊天**。若仍然出现，请打开控制台并反馈 `Support chat crashed` 错误。 |
