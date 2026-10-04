import type { SupportChatRuntimeConfig } from "@/lib/sdk/config-types"

export function readSupportChatRuntimeConfig(): SupportChatRuntimeConfig {
  if (typeof window === "undefined") {
    return {
      channelId: "",
      baseUrl: "",
      apiBaseUrl: "",
    }
  }

  const query = new URLSearchParams(window.location.search)
  const fallback: SupportChatRuntimeConfig = {
    channelId:
      query.get("channelId") ??
      process.env.NEXT_PUBLIC_OPEN_IM_CHANNEL_ID?.trim() ??
      "",
    baseUrl:
      query.get("baseUrl") ??
      process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ??
      window.location.origin,
    apiBaseUrl:
      query.get("apiBaseUrl") ??
      process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ??
      undefined,
    externalId: query.get("externalId") ?? undefined,
    externalIdSignature: query.get("externalIdSignature") ?? undefined,
    externalIdSignedAt: Number(query.get("externalIdSignedAt")) || undefined,
    externalName: query.get("externalName") ?? undefined,
    userToken: query.get("userToken") ?? undefined,
    title: query.get("title") ?? undefined,
    subtitle: query.get("subtitle") ?? undefined,
    position: (query.get("position") as "left" | "right" | null) ?? undefined,
    themeColor: query.get("themeColor") ?? undefined,
    width: query.get("width") ?? undefined,
  }

  if (window.__CS_AI_AGENT_WIDGET_CONFIG__) {
    return window.__CS_AI_AGENT_WIDGET_CONFIG__
  }
  if (window.AgentDeskConfig) {
    // The signer functions cannot cross into the iframe; their resolved values
    // arrive in the query string and in the init payload instead.
    const hostConfig = { ...window.AgentDeskConfig } as Record<string, unknown>
    delete hostConfig.getUserToken
    delete hostConfig.signExternalId
    return {
      ...fallback,
      ...hostConfig,
    } as SupportChatRuntimeConfig
  }
  return fallback
}

export function setSupportChatRuntimeConfig(config: SupportChatRuntimeConfig) {
  if (typeof window === "undefined") {
    return
  }
  window.__CS_AI_AGENT_WIDGET_CONFIG__ = config
}
