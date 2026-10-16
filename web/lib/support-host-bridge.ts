import { setSupportChatRuntimeConfig } from "@/lib/sdk/runtime-config"
import type { SupportChatRuntimeConfig } from "@/lib/sdk/config-types"

type HostBridgeOptions = {
  onInit?: () => void
  onOpen?: () => void
  onMinimize?: () => void
  onMaximizedChange?: (isMaximized: boolean) => void
}

const INIT_MESSAGE_TYPE = "agent-desk:init"
const OPEN_MESSAGE_TYPE = "agent-desk:open"
const MINIMIZE_MESSAGE_TYPE = "agent-desk:minimize"
const MAXIMIZED_MESSAGE_TYPE = "agent-desk:maximized"
const READY_MESSAGE_TYPE = "agent-desk:ready"
const REQUEST_MINIMIZE_MESSAGE_TYPE = "agent-desk:request-minimize"
const REQUEST_CLOSE_MESSAGE_TYPE = "agent-desk:request-close"
const REQUEST_TOGGLE_MAXIMIZE_MESSAGE_TYPE = "agent-desk:request-toggle-maximize"

function trustedParentOrigin() {
  try {
    const value = new URLSearchParams(window.location.search).get("parentOrigin") || document.referrer
    const origin = value ? new URL(value).origin : ""
    return origin && origin !== "null" ? origin : ""
  } catch {
    return ""
  }
}

export function bindSupportHostBridge(options: HostBridgeOptions = {}) {
  if (typeof window === "undefined") {
    return () => undefined
  }

  const parentOrigin = trustedParentOrigin()
  if (parentOrigin && window.parent && window.parent !== window) {
    window.parent.postMessage({ type: READY_MESSAGE_TYPE }, parentOrigin)
  }

  const handleMessage = (event: MessageEvent) => {
    if (
      !parentOrigin ||
      window.parent === window ||
      event.source !== window.parent ||
      event.origin !== parentOrigin
    ) {
      return
    }
    const data = event.data as
      | {
          type?: string
          payload?: SupportChatRuntimeConfig | { isMaximized?: boolean }
        }
      | undefined
    if (!data?.type) {
      return
    }

    if (data.type === INIT_MESSAGE_TYPE && data.payload) {
      setSupportChatRuntimeConfig(data.payload as SupportChatRuntimeConfig)
      options.onInit?.()
      return
    }

    if (data.type === OPEN_MESSAGE_TYPE) {
      options.onOpen?.()
      return
    }

    if (data.type === MINIMIZE_MESSAGE_TYPE) {
      options.onMinimize?.()
      return
    }

    if (data.type === MAXIMIZED_MESSAGE_TYPE) {
      const payload = data.payload as { isMaximized?: boolean } | undefined
      options.onMaximizedChange?.(Boolean(payload?.isMaximized))
    }
  }

  window.addEventListener("message", handleMessage)
  return () => window.removeEventListener("message", handleMessage)
}

function postToParent(type: string) {
  if (typeof window === "undefined") {
    return
  }
  if (window.parent && window.parent !== window) {
    const parentOrigin = trustedParentOrigin()
    if (parentOrigin) {
      window.parent.postMessage({ type }, parentOrigin)
    }
  }
}

export function requestSupportHostMinimize() {
  postToParent(REQUEST_MINIMIZE_MESSAGE_TYPE)
}

export function requestSupportHostClose() {
  postToParent(REQUEST_CLOSE_MESSAGE_TYPE)
}

export function requestSupportHostToggleMaximize() {
  postToParent(REQUEST_TOGGLE_MAXIMIZE_MESSAGE_TYPE)
}
