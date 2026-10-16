# Embedding the Support Widget

The widget adds a chat launcher to any web page. It loads a single script, shows a floating button, and opens the support chat in an iframe. Visitors can keep several requests open and switch between them.

[Русский](./embedding.ru-RU.md) | [简体中文](./embedding.zh-CN.md)

## Quick start

Replace `https://support.example.com` with the address of your deployment and `YOUR_CHANNEL_ID` with the channel ID from the dashboard.

```html
<script>
  window.AgentDeskConfig = {
    channelId: "YOUR_CHANNEL_ID",
  }
</script>
<script src="https://support.example.com/sdk/agent-desk-sdk.min.js" async></script>
```

Put both tags before `</body>`. The `AgentDeskConfig` block must run **before** the SDK script. The widget then mounts itself and the launcher appears in the bottom corner.

Title, subtitle, theme color, position and width are read from the channel settings in the dashboard, so you normally do not set them in code.

## Configuration

| Option | Type | Default | Description |
| --- | --- | --- | --- |
| `channelId` | `string` | – | **Required.** The channel the widget belongs to. |
| `baseUrl` | `string` | script origin | Address of the Agent Desk server. |
| `apiBaseUrl` | `string` | `baseUrl` | Use when the API is served from a different origin. |
| `widgetBaseUrl` | `string` | script origin | Where the chat page is loaded from. |
| `language` | `"zh-CN" \| "en-US"` | server setting | Interface language. Any `en*` value selects English. |
| `position` | `"left" \| "right"` | channel setting | Side of the screen for the launcher and window. |
| `themeColor` | `string` | channel setting | Accent color, for example `#0f6cbd`. |
| `width` | `string` | `380px` | Width of the chat window on desktop. |
| `title`, `subtitle` | `string` | channel setting | Header texts. |
| `externalId` | `string` | browser-local ID | Stable ID of your own user. |
| `signExternalId` | `(id) => {signature, issuedAt}` | – | Proves `externalId` is genuine. See [Identified visitors](#identified-visitors). |
| `externalName` | `string` | – | Display name, used when the visitor is first created. |
| `getUserToken` | `() => string \| Promise<string>` | – | Returns a JWT issued by your system. |

On screens up to 480px wide the chat opens full screen.

## JavaScript API

```js
AgentDeskWidget.open()      // Promise<void>, opens the chat
AgentDeskWidget.close()     // minimizes the chat
AgentDeskWidget.mount(cfg)  // mounts with an explicit config
AgentDeskWidget.destroy()   // removes launcher and chat
AgentDeskWidget.getChatUrl() // Promise<string>, URL for a standalone page or new tab
```

Open the chat from your own button:

```html
<button onclick="AgentDeskWidget.open()">Contact support</button>
```

## React and Next.js

Load the script once on the client. In Next.js (App Router):

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

Config that contains functions (`signExternalId`, `getUserToken`) cannot be serialized. Mount it manually instead:

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

## Identified visitors

By default a visitor is an anonymous guest identified by a browser-local ID. To link chats to your own users, pass `externalId` **and** a signature, so nobody can impersonate another user:

```
signature = hex(HMAC-SHA256(identitySecret, channelId + "\n" + externalId + "\n" + issuedAt))
```

`issuedAt` is the current Unix time in seconds. The server only accepts a signature younger than `identity.maxAgeMinutes`, so the widget requests a fresh one every time the chat opens.

> [!IMPORTANT]
> Compute the signature on **your server**. The identity secret must never be shipped to the browser.

Node.js endpoint:

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

Go:

```go
func sign(secret, channelID, externalID string) (string, int64) {
    issuedAt := time.Now().Unix()
    mac := hmac.New(sha256.New, []byte(secret))
    fmt.Fprintf(mac, "%s\n%s\n%d", channelID, externalID, issuedAt)
    return hex.EncodeToString(mac.Sum(nil)), issuedAt
}
```

Note that the endpoint uses your logged-in user, not a value from the query string, as the `externalId` it signs. An unsigned or invalid pair never breaks the widget: the visitor simply stays a guest.

## Content Security Policy and CORS

If your site sets a CSP, allow the Agent Desk origin:

```
script-src https://support.example.com;
frame-src  https://support.example.com;
connect-src https://support.example.com;
```

When the widget and API live on different origins, add your site to the allowed origins of the deployment so the config request succeeds.

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| No launcher appears | `channelId` is missing or `AgentDeskConfig` is defined after the SDK script. Check the browser console for `channelId is required`. |
| Launcher appears, chat stays blank | The browser blocks the iframe. Check CSP `frame-src` and any `X-Frame-Options` set by a proxy on the Agent Desk origin. |
| Visitor always shows as guest | The signature is missing, expired or computed with the wrong secret or payload order. Check server clock drift against `identity.maxAgeMinutes`. |
| Wrong language | Set `language` explicitly, otherwise the server default is used. |
| Widget duplicated in a single-page app | Call `AgentDeskWidget.destroy()` when the component unmounts. |
| Chat shows "Something went wrong" | Choose **Reload chat**. If it persists, open the console and report the logged `Support chat crashed` error. |
