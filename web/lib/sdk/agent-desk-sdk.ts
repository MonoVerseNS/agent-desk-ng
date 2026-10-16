import type {
  AgentDeskConfig,
  AgentDeskWidget,
  SupportChatRuntimeConfig,
} from "./config-types"

type NormalizedAgentDeskConfig = AgentDeskConfig & {
  baseUrl: string
  channelId: string
  language: string
  position: "left" | "right"
  themeColor: string
  width: string
}

/** A host-signed visitor identity, resolved fresh on every open. */
type ResolvedExternalIdentity = {
  signature: string
  issuedAt: number
}

type WidgetState = {
  button: HTMLButtonElement | null
  frame: HTMLIFrameElement | null
  frameLoaded: boolean
  frameReady: boolean
  initSent: boolean
  isOpen: boolean
  opening: boolean
  isMaximized: boolean
  mounted: boolean
  mountGeneration: number
  openGeneration: number
  cancelOpen: (() => void) | null
  configLoading: boolean
  configPromise: Promise<void> | null
  frameHideTimer: number | null
  config: NormalizedAgentDeskConfig | null
  frameConfig: SupportChatRuntimeConfig | null
  frameUrl: URL | null
  animationDuration: number
  messageListener: ((event: MessageEvent) => void) | null
  resizeListener: (() => void) | null
}

type WidgetConfigResponse = {
  success?: boolean
  data?: Partial<Pick<
    AgentDeskConfig,
    "title" | "subtitle" | "themeColor" | "position" | "width"
  >>
}

type PublicConfigResponse = {
  success?: boolean
  data?: {
    language?: string
  }
}

function normalizeWidgetLanguage(language: string | undefined) {
  const lang = String(language || "").toLowerCase()
  if (lang.startsWith("ru")) return "ru-RU"
  if (lang.startsWith("en")) return "en-US"
  return "zh-CN"
}

function getDefaultWidgetTitle(config?: NormalizedAgentDeskConfig | null) {
  const lang = normalizeWidgetLanguage(config?.language)
  if (lang === "ru-RU") return "Поддержка"
  if (lang === "en-US") return "Support"
  return "\u5728\u7ebf\u5ba2\u670d"
}

function getLauncherText(config?: NormalizedAgentDeskConfig | null) {
  const lang = normalizeWidgetLanguage(config?.language)
  if (lang === "ru-RU") return "Чат"
  if (lang === "en-US") return "Support"
  return "\u5ba2\u670d"
}

type FrameMessage =
  | { type: "agent-desk:init"; payload: SupportChatRuntimeConfig }
  | { type: "agent-desk:open" }
  | { type: "agent-desk:minimize" }
  | { type: "agent-desk:maximized"; payload: { isMaximized: boolean } }

;(function () {
  const DEFAULT_CONFIG: Pick<
    NormalizedAgentDeskConfig,
    "language" | "position" | "themeColor" | "width"
  > = {
    language: "zh-CN",
    position: "right",
    themeColor: "#0f6cbd",
    width: "380px",
  }

  // Capture the SDK origin while currentScript still refers to this script.
  const currentScript = document.currentScript as HTMLScriptElement | null
  const scriptBaseUrl = currentScript?.src
    ? currentScript.src.replace(/\/sdk\/agent-desk-sdk\.min\.js(?:\?.*)?$/, "")
    : ""
  const existingState = window.__CS_AI_AGENT_WIDGET_STATE__ as WidgetState | undefined
  // Re-evaluating the script must not leave callbacks from an older instance.
  if (existingState) window.AgentDeskWidget?.destroy()
  const state: WidgetState =
    existingState || {
      button: null,
      frame: null,
      frameLoaded: false,
      frameReady: false,
      initSent: false,
      isOpen: false,
      opening: false,
      isMaximized: false,
      mounted: false,
      mountGeneration: 0,
      openGeneration: 0,
      cancelOpen: null,
      configLoading: false,
      configPromise: null,
      frameHideTimer: null,
      config: null,
      frameConfig: null,
      frameUrl: null,
      animationDuration: 260,
      messageListener: null,
      resizeListener: null,
    }
  if (!existingState) {
    window.__CS_AI_AGENT_WIDGET_STATE__ = state
  }

  function normalizeConfig(config?: AgentDeskConfig): NormalizedAgentDeskConfig {
    const merged: Record<string, unknown> = { ...DEFAULT_CONFIG, ...(config || {}) }
    merged.baseUrl = String(merged.baseUrl || window.location.origin).replace(/\/$/, "")
    if (merged.apiBaseUrl) {
      merged.apiBaseUrl = String(merged.apiBaseUrl).replace(/\/$/, "")
    } else {
      delete merged.apiBaseUrl
    }
    merged.channelId = String(merged.channelId || "")
    merged.language = normalizeWidgetLanguage(String(merged.language || "zh-CN"))
    if (merged.externalId) {
      merged.externalId = String(merged.externalId)
    }
    if (typeof merged.getUserToken !== "function") {
      delete merged.getUserToken
    }
    return merged as NormalizedAgentDeskConfig
  }

  function resolveWidgetBaseUrl(config: NormalizedAgentDeskConfig) {
    return String(config.widgetBaseUrl || scriptBaseUrl || config.baseUrl || window.location.origin).replace(/\/$/, "")
  }

  function resolveConfig(config: AgentDeskConfig) {
    const normalized = normalizeConfig(config)
    normalized.widgetBaseUrl = resolveWidgetBaseUrl(normalized)
    if (!config.baseUrl) normalized.baseUrl = normalized.widgetBaseUrl
    return normalized
  }

  function createFrameUrl(
    config: NormalizedAgentDeskConfig,
    userToken: string,
    identity: ResolvedExternalIdentity
  ) {
    const widgetBaseUrl = resolveWidgetBaseUrl(config)
    const frameUrl = new URL(`${widgetBaseUrl}/support/chat/`)
    frameUrl.searchParams.set("channelId", config.channelId)
    frameUrl.searchParams.set("baseUrl", config.baseUrl)
    frameUrl.searchParams.set("parentOrigin", window.location.origin)
    if (config.requireUserToken) frameUrl.searchParams.set("requireUserToken", "true")
    if (config.apiBaseUrl) frameUrl.searchParams.set("apiBaseUrl", config.apiBaseUrl)
    if (config.externalId) frameUrl.searchParams.set("externalId", config.externalId)
    if (identity.signature) frameUrl.searchParams.set("externalIdSignature", identity.signature)
    if (identity.issuedAt) frameUrl.searchParams.set("externalIdSignedAt", String(identity.issuedAt))
    if (config.externalName) frameUrl.searchParams.set("externalName", config.externalName)
    if (userToken) frameUrl.searchParams.set("userToken", userToken)
    return frameUrl
  }

  function createFrameConfig(
    config: NormalizedAgentDeskConfig,
    userToken: string,
    identity: ResolvedExternalIdentity
  ): SupportChatRuntimeConfig {
    const payload = { ...config } as Record<string, unknown>
    // Functions cannot cross the postMessage boundary; their resolved values travel instead.
    delete payload.getUserToken
    delete payload.signExternalId
    if (userToken) {
      return { ...payload, userToken } as SupportChatRuntimeConfig
    }
    if (identity.signature) {
      return {
        ...payload,
        externalIdSignature: identity.signature,
        externalIdSignedAt: identity.issuedAt,
      } as SupportChatRuntimeConfig
    }
    return payload as SupportChatRuntimeConfig
  }

  function resolveUserToken(config: NormalizedAgentDeskConfig) {
    if (typeof config?.getUserToken !== "function") {
      if (config.requireUserToken) return Promise.reject(new Error("Support user token is required"))
      return Promise.resolve("")
    }
    try {
      return Promise.resolve(config.getUserToken()).then((token) => {
        const value = String(token || "").trim()
        if (config.requireUserToken && !value) throw new Error("Support user token is required")
        return value
      })
    } catch (error) {
      return Promise.reject(error)
    }
  }

  // Resolved per open(), like getUserToken, so the pair is always fresh and a
  // long-lived host page does not keep replaying an expired signature. A missing
  // signer simply means the visitor stays an anonymous guest.
  function resolveExternalIdentity(config: NormalizedAgentDeskConfig): Promise<ResolvedExternalIdentity> {
    if (!config?.externalId || typeof config.signExternalId !== "function") {
      return Promise.resolve({ signature: "", issuedAt: 0 })
    }
    try {
      return Promise.resolve(config.signExternalId(config.externalId)).then((signed) => ({
        signature: String(signed?.signature || "").trim(),
        issuedAt: Number(signed?.issuedAt) || 0,
      }))
    } catch (error) {
      return Promise.reject(error)
    }
  }

  function prepareFrame(config: NormalizedAgentDeskConfig) {
    // Resolve against one snapshot. A later mount must never receive a token or
    // signature requested for the previous channel or visitor.
    return Promise.all([resolveUserToken(config), resolveExternalIdentity(config)]).then(
      ([userToken, identity]) => ({
        frameUrl: createFrameUrl(config, userToken, identity),
        frameConfig: createFrameConfig(config, userToken, identity),
      })
    )
  }

  function mergeWidgetConfig(
    config: NormalizedAgentDeskConfig,
    remoteConfig?: WidgetConfigResponse["data"]
  ) {
    if (!remoteConfig) {
      return config
    }
    const merged: NormalizedAgentDeskConfig = { ...config }
    const remoteKeys = ["title", "subtitle", "themeColor", "position", "width"] as const
    remoteKeys.forEach((key) => {
      const value = remoteConfig[key]
      if (value !== undefined && value !== null) {
        ;(merged[key] as typeof value) = value
      }
    })
    return merged
  }

  function fetchConfigPayload<T>(url: string, options: RequestInit): Promise<T> {
    const controller = typeof AbortController === "function" ? new AbortController() : null
    // Remote defaults are optional. A slow config service must not prevent the
    // host from opening with its own configuration indefinitely.
    return new Promise<T>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        controller?.abort()
        reject(new Error("Widget configuration request timed out"))
      }, 5000)
      Promise.resolve()
        .then(() => fetch(url, { ...options, signal: controller?.signal }))
        .then((response) => response.json() as Promise<T>)
        .then(
          (payload) => {
            window.clearTimeout(timer)
            resolve(payload)
          },
          (error) => {
            window.clearTimeout(timer)
            reject(error)
          }
        )
    })
  }

  function fetchWidgetConfig(config: NormalizedAgentDeskConfig) {
    const baseUrl = String(config.apiBaseUrl || config.baseUrl || "").replace(/\/$/, "")
    if (!baseUrl || !config.channelId || typeof fetch !== "function") {
      return Promise.resolve(config)
    }
    const url = `${baseUrl}/api/channel/config?channelId=${encodeURIComponent(config.channelId)}`
    return fetchConfigPayload<WidgetConfigResponse>(url, {
      method: "GET",
      cache: "no-store",
      headers: {
        "X-Channel-Id": config.channelId,
      },
    })
      .then((payload) => {
        if (!payload || payload.success === false) {
          return config
        }
        return mergeWidgetConfig(config, payload.data || {})
      })
      .catch(() => config)
  }

  function fetchPublicConfig(config: NormalizedAgentDeskConfig) {
    const baseUrl = String(config.apiBaseUrl || config.baseUrl || "").replace(/\/$/, "")
    if (!baseUrl || typeof fetch !== "function") {
      return Promise.resolve(config)
    }
    return fetchConfigPayload<PublicConfigResponse>(`${baseUrl}/api/config`, {
      method: "GET",
      cache: "no-store",
    })
      .then((payload) => {
        if (!payload || payload.success === false) {
          return config
        }
        return normalizeConfig({
          ...config,
          language: payload.data?.language || config.language,
        })
      })
      .catch(() => config)
  }

  function clearFrameTimers() {
    if (state.frameHideTimer !== null) {
      window.clearTimeout(state.frameHideTimer)
      state.frameHideTimer = null
    }
  }

  function applyFrameLayout() {
    const frame = state.frame
    const config = state.config
    if (!frame || !config) {
      return
    }

    frame.style.position = "fixed"
    frame.style.border = "0"
    frame.style.overflow = "hidden"
    frame.style.background = "#fff"
    frame.style.zIndex = "2147483000"
    frame.style.boxShadow =
      "0 0 0 1px rgba(15, 35, 65, 0.06), 0 4px 12px rgba(15, 35, 65, 0.08), 0 24px 64px rgba(15, 35, 65, 0.22)"
    frame.style.willChange = "top,right,bottom,left,width,height,opacity,transform,border-radius"
    frame.style.transition =
      "top 260ms cubic-bezier(0.22, 1, 0.36, 1), right 260ms cubic-bezier(0.22, 1, 0.36, 1), bottom 260ms cubic-bezier(0.22, 1, 0.36, 1), left 260ms cubic-bezier(0.22, 1, 0.36, 1), width 260ms cubic-bezier(0.22, 1, 0.36, 1), height 260ms cubic-bezier(0.22, 1, 0.36, 1), opacity 220ms ease, transform 260ms cubic-bezier(0.22, 1, 0.36, 1), border-radius 260ms cubic-bezier(0.22, 1, 0.36, 1), box-shadow 260ms ease"
    frame.style.transformOrigin =
      config.position === "left" ? "left bottom" : "right bottom"

    // Phones get the full viewport instead of a floating card that squeezes the chat.
    const isCompact =
      typeof window.innerWidth === "number" && window.innerWidth > 0 && window.innerWidth <= 480
    if (state.isMaximized || isCompact) {
      frame.style.top = "max(12px, env(safe-area-inset-top))"
      frame.style.right = "max(12px, env(safe-area-inset-right))"
      frame.style.bottom = "max(12px, env(safe-area-inset-bottom))"
      frame.style.left = "max(12px, env(safe-area-inset-left))"
      frame.style.width =
        "calc(100vw - max(12px, env(safe-area-inset-left)) - max(12px, env(safe-area-inset-right)))"
      frame.style.maxWidth = "none"
      frame.style.height =
        "calc(100dvh - max(12px, env(safe-area-inset-top)) - max(12px, env(safe-area-inset-bottom)))"
      frame.style.borderRadius = "16px"
      return
    }

    frame.style.top = ""
    frame.style.bottom = "max(88px, calc(84px + env(safe-area-inset-bottom)))"
    frame.style.right = config.position === "left" ? "" : "max(12px, env(safe-area-inset-right))"
    frame.style.left = config.position === "left" ? "max(12px, env(safe-area-inset-left))" : ""
    frame.style.width = config.width || "380px"
    frame.style.maxWidth =
      "calc(100vw - max(12px, env(safe-area-inset-left)) - max(12px, env(safe-area-inset-right)))"
    frame.style.height =
      "min(760px, calc(100dvh - max(104px, calc(88px + env(safe-area-inset-bottom))) - max(12px, env(safe-area-inset-top))))"
    frame.style.borderRadius = "16px"
  }

  function postToFrame(message: FrameMessage) {
    if (!state.frame?.contentWindow || !state.frameUrl) {
      return
    }
    try {
      state.frame.contentWindow.postMessage(message, state.frameUrl.origin)
    } catch (error) {
      console.error("[agent-desk-widget] postMessage failed", error)
    }
  }

  function flushFrameState() {
    if (!state.frame || !state.frameLoaded || !state.frameReady || !state.frameConfig) {
      return
    }

    if (!state.initSent) {
      state.initSent = true
      postToFrame({
        type: "agent-desk:init",
        payload: state.frameConfig,
      })
    }

    postToFrame({ type: state.isOpen ? "agent-desk:open" : "agent-desk:minimize" })
    postToFrame({
      type: "agent-desk:maximized",
      payload: { isMaximized: state.isMaximized },
    })
  }

  function syncLauncherState() {
    state.button?.setAttribute("aria-expanded", String(state.isOpen || state.opening))
    state.button?.setAttribute("aria-busy", String(state.opening))
  }

  function syncFrameVisibility() {
    syncLauncherState()
    const frame = state.frame
    if (!frame) {
      return
    }
    clearFrameTimers()
    applyFrameLayout()
    frame.style.display = "block"

    if (state.isOpen) {
      frame.style.visibility = "visible"
      frame.style.pointerEvents = "auto"
      state.frameHideTimer = window.setTimeout(() => {
        if (state.frame !== frame || !state.isOpen || !state.mounted) {
          return
        }
        frame.style.opacity = "1"
        frame.style.transform = "translate3d(0, 0, 0) scale(1)"
      }, 16)
      flushFrameState()
      return
    }

    frame.style.pointerEvents = "none"
    frame.style.opacity = "0"
    frame.style.transform = state.isMaximized
      ? "translate3d(0, 10px, 0) scale(0.985)"
      : "translate3d(0, 16px, 0) scale(0.96)"
    state.frameHideTimer = window.setTimeout(() => {
      if (state.frame !== frame || state.isOpen || !state.mounted) {
        return
      }
      frame.style.visibility = "hidden"
    }, state.animationDuration)
    flushFrameState()
  }

  function removeFrame() {
    clearFrameTimers()
    if (state.frame?.parentNode) state.frame.parentNode.removeChild(state.frame)
    state.frame = null
    state.frameLoaded = false
    state.frameReady = false
    state.initSent = false
  }

  function createFrame() {
    if (state.frame) {
      return state.frame
    }
    if (!state.frameUrl || !state.config || !state.mounted) {
      return null
    }

    const frame = document.createElement("iframe")
    state.frame = frame
    frame.dataset.agentDeskWidget = "frame"
    frame.title = state.config.title || getDefaultWidgetTitle(state.config)
    frame.src = state.frameUrl.toString()
    applyFrameLayout()
    frame.style.display = "block"
    frame.style.visibility = "hidden"
    frame.style.pointerEvents = "none"
    frame.style.opacity = "0"
    frame.style.transform = "translate3d(0, 18px, 0) scale(0.96)"
    frame.addEventListener("load", () => {
      if (state.frame !== frame || !state.mounted) return
      state.frameLoaded = true
      syncFrameVisibility()
    })

    document.body.appendChild(frame)
    return frame
  }

  function handleWindowMessage(event: MessageEvent) {
    if (
      !state.mounted ||
      !state.frame ||
      !state.frameUrl ||
      event.source !== state.frame.contentWindow ||
      event.origin !== state.frameUrl.origin
    ) {
      return
    }

    if (!event.data || typeof event.data !== "object") return
    const data = event.data as { type?: string }
    if (data.type === "agent-desk:ready") {
      state.frameReady = true
      flushFrameState()
      return
    }

    if (data.type === "agent-desk:request-minimize" || data.type === "agent-desk:request-close") {
      closeWidget()
      return
    }

    if (data.type === "agent-desk:request-toggle-maximize") {
      state.isMaximized = !state.isMaximized
      syncFrameVisibility()
    }
  }

  function createLauncher() {
    if (state.button) {
      return state.button
    }

    const config = state.config
    if (!config) {
      return null
    }
    const button = document.createElement("button")
    const icon = document.createElementNS("http://www.w3.org/2000/svg", "svg")
    const iconPaths = ["M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"]
    const text = document.createElement("span")
    const themeColor = config.themeColor || "#0f6cbd"
    const reduceMotion =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches
    const restShadow = "0 1px 2px rgba(15, 35, 65, 0.16), 0 8px 24px rgba(15, 35, 65, 0.22)"
    const hoverShadow = "0 2px 4px rgba(15, 35, 65, 0.18), 0 12px 32px rgba(15, 35, 65, 0.28)"
    button.type = "button"
    button.dataset.agentDeskWidget = "launcher"
    button.setAttribute("aria-label", config.title || getDefaultWidgetTitle(config))
    icon.setAttribute("viewBox", "0 0 24 24")
    icon.setAttribute("fill", "none")
    icon.setAttribute("stroke", "currentColor")
    icon.setAttribute("stroke-width", "2")
    icon.setAttribute("stroke-linecap", "round")
    icon.setAttribute("stroke-linejoin", "round")
    icon.setAttribute("aria-hidden", "true")
    icon.style.width = "26px"
    icon.style.height = "26px"
    icon.style.flex = "0 0 auto"
    iconPaths.forEach((pathData) => {
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path")
      path.setAttribute("d", pathData)
      icon.appendChild(path)
    })
    // Visually hidden: the icon carries the meaning, the label serves assistive tech.
    text.textContent = getLauncherText(config)
    text.style.position = "absolute"
    text.style.width = "1px"
    text.style.height = "1px"
    text.style.overflow = "hidden"
    text.style.clip = "rect(0 0 0 0)"
    text.style.whiteSpace = "nowrap"
    button.style.position = "fixed"
    button.style.bottom = "max(20px, env(safe-area-inset-bottom))"
    button.style.right = config.position === "left" ? "" : "max(20px, env(safe-area-inset-right))"
    button.style.left = config.position === "left" ? "max(20px, env(safe-area-inset-left))" : ""
    button.style.zIndex = "2147483000"
    button.style.display = "inline-flex"
    button.style.alignItems = "center"
    button.style.justifyContent = "center"
    button.style.width = "56px"
    button.style.height = "56px"
    button.style.border = "0"
    button.style.borderRadius = "999px"
    button.style.padding = "0"
    button.style.background = themeColor
    button.style.color = "#fff"
    button.style.boxShadow = restShadow
    button.style.cursor = "pointer"
    button.style.outline = "none"
    button.style.transition = reduceMotion
      ? "none"
      : "transform 180ms cubic-bezier(0.22, 1, 0.36, 1), box-shadow 180ms ease"
    button.appendChild(icon)
    button.appendChild(text)

    const lift = () => {
      button.style.transform = "translateY(-2px)"
      button.style.boxShadow = hoverShadow
    }
    const rest = () => {
      button.style.transform = "none"
      button.style.boxShadow = restShadow
    }
    button.addEventListener("mouseenter", lift)
    button.addEventListener("mouseleave", rest)
    button.addEventListener("focus", () => {
      button.style.boxShadow = `${hoverShadow}, 0 0 0 3px ${themeColor}55`
    })
    button.addEventListener("blur", rest)

    button.addEventListener("click", () => {
      if (state.isOpen || state.opening) {
        closeWidget()
        return
      }

      // openWidget logs active failures; native clicks have no Promise caller.
      void openWidget().catch(() => {})
    })

    document.body.appendChild(button)
    state.button = button
    syncLauncherState()
    return button
  }

  function mount(config?: AgentDeskConfig) {
    const rawConfig = { ...(config || window.AgentDeskConfig || { channelId: "" }) }
    // A remount replaces the previous UI and invalidates every pending request.
    // Conversation history belongs to the chat, not this disposable host UI.
    destroy()
    state.config = resolveConfig(rawConfig)
    if (!state.config.channelId) {
      console.error("[agent-desk-widget] channelId is required")
      return
    }

    state.mounted = true
    state.messageListener = handleWindowMessage
    state.resizeListener = applyFrameLayout
    window.addEventListener("message", state.messageListener)
    window.addEventListener("resize", state.resizeListener)
    const mountGeneration = state.mountGeneration
    state.configLoading = true
    const publicConfig = rawConfig.language
      ? Promise.resolve(state.config)
      : fetchPublicConfig(state.config)
    state.configPromise = publicConfig
      .then((nextConfig) => {
        if (!state.mounted || state.mountGeneration !== mountGeneration) return null
        return fetchWidgetConfig(nextConfig)
      })
      .then((nextConfig) => {
        if (!nextConfig || !state.mounted || state.mountGeneration !== mountGeneration) return
        state.configLoading = false
        state.config = normalizeConfig(nextConfig)
        createLauncher()
      })
  }

  function destroy() {
    state.mounted = false
    state.mountGeneration = (state.mountGeneration || 0) + 1
    cancelPendingOpen()
    if (state.messageListener) window.removeEventListener("message", state.messageListener)
    if (state.resizeListener) window.removeEventListener("resize", state.resizeListener)
    state.messageListener = null
    state.resizeListener = null
    removeFrame()
    if (state.button?.parentNode) {
      state.button.parentNode.removeChild(state.button)
    }
    state.button = null
    state.isOpen = false
    state.isMaximized = false
    state.configLoading = false
    state.configPromise = null
    state.frameConfig = null
    state.frameUrl = null
  }

  function cancelPendingOpen() {
    state.openGeneration = (state.openGeneration || 0) + 1
    state.cancelOpen?.()
    state.cancelOpen = null
    state.opening = false
  }

  function closeWidget() {
    cancelPendingOpen()
    state.isOpen = false
    syncFrameVisibility()
  }

  async function openWidget() {
    if (!state.mounted) mount(state.config || window.AgentDeskConfig)
    if (!state.config?.channelId || !state.mounted) return

    cancelPendingOpen()
    const mountGeneration = state.mountGeneration
    const openGeneration = state.openGeneration
    const canceled = new Promise<null>((resolve) => {
      state.cancelOpen = () => resolve(null)
    })
    const isCurrentOpen = () =>
      state.mounted &&
      state.mountGeneration === mountGeneration &&
      state.openGeneration === openGeneration
    state.opening = true
    syncLauncherState()
    try {
      await Promise.race([state.configPromise, canceled])
      if (!isCurrentOpen() || !state.config) return
      const prepared = await Promise.race([prepareFrame({ ...state.config }), canceled])
      // close(), destroy(), and a newer open()/mount() all cancel this open.
      // Do not assign identity, create a frame or flash stale UI afterwards.
      if (!prepared || !isCurrentOpen()) return
      state.frameUrl = prepared.frameUrl
      state.frameConfig = prepared.frameConfig
      if (state.frame && state.frame.src !== prepared.frameUrl.toString()) {
        // A fresh identity needs a new document. Replacing the element also
        // fences delayed load/ready events from its previous document.
        removeFrame()
      }
      if (!state.frame) createFrame()
      if (!state.frame) return
      state.opening = false
      state.isOpen = true
      syncFrameVisibility()
    } catch (error) {
      if (isCurrentOpen()) {
        console.error("[agent-desk-widget] open failed", error)
        throw error
      }
    } finally {
      if (isCurrentOpen()) {
        state.cancelOpen = null
        state.opening = false
        syncLauncherState()
      }
    }
  }

  window.AgentDeskWidget = {
    mount,
    destroy,
    open: () => openWidget(),
    close: closeWidget,
    getChatUrl: () => {
      const config = state.config
        ? { ...state.config }
        : resolveConfig(window.AgentDeskConfig || { channelId: "" })
      if (!config.channelId) {
        return Promise.reject(new Error("channelId is required"))
      }
      // A standalone link must not change the active frame's URL or identity.
      return prepareFrame(config).then(({ frameUrl }) => frameUrl.toString())
    },
  } satisfies AgentDeskWidget

  if (window.AgentDeskConfig) {
    mount(window.AgentDeskConfig)
  }
})()
