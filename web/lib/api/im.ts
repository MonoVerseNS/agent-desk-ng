import { request } from "@/lib/api/client"
import { translateCurrentMessage } from "@/i18n/messages"
import { readSupportChatRuntimeConfig } from "@/lib/sdk/runtime-config"
import { generateUUID, parseApiTimestamp } from "@/lib/utils"

export type Paging = {
  page: number
  limit: number
  total: number
}

export type PageResult<T> = {
  results?: T[] | null
  page: Paging
  cursor?: string
  hasMore?: boolean
}

export type ImConversationTag = {
  id: number
  name: string
  color: string
}

export type ImConversationParticipant = {
  id: number
  participantType: string
  participantId: number
  externalParticipantId?: string
  joinedAt?: string
  leftAt?: string
  status: number
}

export interface SubmitSatisfactionParams {
  conversationId: number
  score: number
  feedback?: string
}

export interface ConversationSatisfactionResponse {
  id: number
  conversationId: number
  score: number
  feedback: string
  createdAt: string
}

export type ImConversation = {
  id: number
  channelId: number
  customerName: string
  subject?: string
  status: number
  serviceMode: number
  priority: number
  currentAssigneeId: number
  currentAssigneeName?: string
  currentTeamId?: number
  lastMessageId: number
  lastMessageAt?: string
  lastActiveAt?: string
  lastMessageSummary?: string
  customerUnreadCount: number
  agentUnreadCount: number
  customerLastReadMessageId: number
  customerLastReadAt?: string
  agentLastReadMessageId: number
  agentLastReadAt?: string
  closedAt?: string
  tags?: ImConversationTag[]
  participants?: ImConversationParticipant[]
  satisfaction?: ConversationSatisfactionResponse | null
}

export type ImConversationDetail = ImConversation
export type ConversationResponse = ImConversation

export type ImMessage = {
  id: number
  conversationId: number
  workflowRunId?: number
  clientMsgId?: string
  senderType: string
  senderId: number
  senderName?: string
  senderAvatar?: string
  messageType: string
  content: string
  payload?: string
  sendStatus: number
  sentAt?: string
  deliveredAt?: string
  readAt?: string
  customerRead: boolean
  customerReadAt?: string
  agentRead: boolean
  agentReadAt?: string
  recalledAt?: string
  quotedMessageId?: number
}

export type ImAsset = {
  id: number
  assetId: string
  provider: string
  storageKey: string
  filename: string
  fileSize: number
  mimeType: string
  status: number
  url: string
  createdAt: string
  updatedAt: string
  createUserId: number
  createUserName: string
  updateUserId: number
  updateUserName: string
}

export type WidgetConfigResponse = {
  channelId?: string
  channelType?: string
  userToken?: string
  title?: string
  subtitle?: string
  themeColor?: string
  position?: "left" | "right"
  width?: string
  promptStarters?: string[]
}

export type ImWidgetConfig = WidgetConfigResponse

export type ImCustomerSessionCustomer = {
  id: number
  name: string
}

export type ImCustomerSessionExchangeResponse = {
  customerSessionToken: string
  expiresAt: string
  identityKey: string
  customer: ImCustomerSessionCustomer
}

export type ImCustomerSession = ImCustomerSessionExchangeResponse & {
  channelId: string
  baseUrl: string
}

const GUEST_STORAGE_KEY = "cs_ai_agent_im_guest_id"
const CUSTOMER_SESSION_STORAGE_KEY = "cs_ai_agent_customer_session"
const CUSTOMER_SESSION_TOKEN_HEADER = "X-Customer-Session-Token"
const CUSTOMER_SESSION_EXPIRES_HEADER = "X-Customer-Session-Expires-At"
const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL?.trim() || ""
const OPEN_IM_CHANNEL_ID =
  process.env.NEXT_PUBLIC_OPEN_IM_CHANNEL_ID?.trim() || ""

let entryIdentityExchangeKey = ""

function buildGuestId() {
  return `guest_${generateUUID()}`
}

export function getGuestId() {
  if (typeof window === "undefined") {
    return ""
  }
  const existing = window.localStorage.getItem(GUEST_STORAGE_KEY)?.trim()
  if (existing) {
    return existing
  }
  const guestId = buildGuestId()
  window.localStorage.setItem(GUEST_STORAGE_KEY, guestId)
  return guestId
}

function getRuntimeImConfig() {
  const widgetConfig = readSupportChatRuntimeConfig()
  const baseUrl = (widgetConfig.apiBaseUrl || widgetConfig.baseUrl || API_BASE_URL)
    .trim()
    .replace(/\/$/, "")
  return {
    baseUrl,
    channelId: widgetConfig.channelId || OPEN_IM_CHANNEL_ID,
    externalId: (widgetConfig.externalId || "").trim(),
    externalIdSignature: (widgetConfig.externalIdSignature || "").trim(),
    externalIdSignedAt: Number(widgetConfig.externalIdSignedAt) || 0,
    externalName: (widgetConfig.externalName || "").trim(),
    userToken: (widgetConfig.userToken || "").trim(),
    requireUserToken: widgetConfig.requireUserToken === true,
  }
}

type RuntimeImConfig = ReturnType<typeof getRuntimeImConfig>

function identityContextKey(config: RuntimeImConfig) {
  return JSON.stringify([
    config.baseUrl,
    config.channelId,
    config.externalId,
    config.userToken,
    config.externalIdSignature,
    config.externalIdSignedAt,
    config.requireUserToken,
  ])
}

function requireEntryToken(config: RuntimeImConfig) {
  if (config.requireUserToken && !config.userToken) {
    throw new Error(translateCurrentMessage("api.supportAuthenticationRequired"))
  }
}

function matchesSessionIdentity(session: ImCustomerSession, config: RuntimeImConfig) {
  if (typeof session.identityKey !== "string") return false
  if (session.baseUrl !== config.baseUrl || session.channelId !== config.channelId) return false
  if (config.userToken || config.requireUserToken) {
    return (
      !!config.userToken &&
      (config.externalId
        ? session.identityKey === `user:${config.externalId}`
        : session.identityKey.startsWith("user:"))
    )
  }
  const externalId = config.externalId || getGuestId()
  return (
    session.identityKey === `guest:${externalId}` ||
    (!!config.externalIdSignature && session.identityKey === `user:${externalId}`)
  )
}

function usableCustomerSession(session: ImCustomerSession | null, config: RuntimeImConfig) {
  if (!session || !isCustomerSessionValid(session) || !matchesSessionIdentity(session, config)) return false
  // A prior iframe's session, especially a prior guest, cannot replace this
  // entry's JWT verification. Signed HMAC entries likewise re-exchange first.
  if ((config.userToken || config.externalIdSignature) && entryIdentityExchangeKey !== identityContextKey(config)) {
    return false
  }
  return true
}

function sessionTokenClaims(token: string): { identityKey?: string; channelCode?: string } | null {
  try {
    const payload = token.split(".")[1]
    if (!payload) return null
    const base64 = payload.replace(/-/g, "+").replace(/_/g, "/")
    const bytes = Uint8Array.from(
      atob(base64.padEnd(Math.ceil(base64.length / 4) * 4, "=")),
      (c) => c.charCodeAt(0)
    )
    return JSON.parse(new TextDecoder().decode(bytes))
  } catch {
    return null
  }
}

function parseExpiresAt(value: string) {
  return parseApiTimestamp(value)
}

function isCustomerSessionValid(
  session: ImCustomerSession | null,
  channelId?: string,
  identityKey?: string
) {
  if (!session?.customerSessionToken || !session.expiresAt) {
    return false
  }
  if (channelId && session.channelId !== channelId) {
    return false
  }
  if (identityKey && session.identityKey !== identityKey) {
    return false
  }
  return parseExpiresAt(session.expiresAt) > Date.now() + 5000
}

export function readCustomerSession(): ImCustomerSession | null {
  if (typeof window === "undefined") {
    return null
  }
  const raw = window.sessionStorage.getItem(CUSTOMER_SESSION_STORAGE_KEY)
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as ImCustomerSession
  } catch {
    window.sessionStorage.removeItem(CUSTOMER_SESSION_STORAGE_KEY)
    return null
  }
}

function writeCustomerSession(session: ImCustomerSession) {
  if (typeof window === "undefined") {
    return
  }
  window.sessionStorage.setItem(CUSTOMER_SESSION_STORAGE_KEY, JSON.stringify(session))
}

export function getCustomerSessionToken() {
  const config = getRuntimeImConfig()
  const session = readCustomerSession()
  return usableCustomerSession(session, config)
    ? session?.customerSessionToken ?? ""
    : ""
}

export function applyCustomerSessionRefresh(payload?: {
  customerSessionToken?: string
  expiresAt?: string
}) {
  const token = payload?.customerSessionToken?.trim()
  const expiresAt = payload?.expiresAt?.trim()
  if (!token || !expiresAt) {
    return
  }
  const current = readCustomerSession()
  const config = getRuntimeImConfig()
  if (!current || !usableCustomerSession(current, config)) return
  // Decode only for cache consistency, never as proof of authenticity. The
  // server still verifies signatures. A late old-account websocket/HTTP refresh
  // must not overwrite the current account's stored customer session.
  const claims = sessionTokenClaims(token)
  if (claims?.identityKey !== current.identityKey || claims?.channelCode !== current.channelId) return
  writeCustomerSession({
    ...current,
    customerSessionToken: token,
    expiresAt,
  })
}

function applyCustomerSessionHeaders(response: Response) {
  applyCustomerSessionRefresh({
    customerSessionToken: response.headers.get(CUSTOMER_SESSION_TOKEN_HEADER) ?? "",
    expiresAt: response.headers.get(CUSTOMER_SESSION_EXPIRES_HEADER) ?? "",
  })
}

function createChannelHeaders() {
  const config = getRuntimeImConfig()
  return {
    "X-Channel-Id": config.channelId,
  }
}

function createExchangeHeaders() {
  const config = getRuntimeImConfig()
  requireEntryToken(config)
  const headers: Record<string, string> = {
    "X-Channel-Id": config.channelId,
  }
  if (config.userToken) {
    headers.Authorization = `Bearer ${config.userToken}`
  } else {
    headers["X-External-Id"] = config.externalId || getGuestId()
    // Only meaningful together with a host-supplied externalId: the browser-local
    // guest id cannot be signed by the host. Without a valid signature the
    // server treats the visitor as anonymous.
    if (config.externalId && config.externalIdSignature && config.externalIdSignedAt) {
      headers["X-External-Id-Signature"] = config.externalIdSignature
      headers["X-External-Id-Signed-At"] = String(config.externalIdSignedAt)
    }
    if (config.externalName) {
      headers["X-External-Name"] = encodeURIComponent(config.externalName)
    }
  }
  return {
    ...headers,
  }
}

function createImHeaders(explicitToken?: string) {
  const sessionToken = explicitToken || getCustomerSessionToken()
  if (!sessionToken) {
    throw new Error(translateCurrentMessage("api.customerSessionNotReady"))
  }
  return {
    ...createChannelHeaders(),
    Authorization: `Bearer ${sessionToken}`,
    ...(explicitToken ? { "X-Customer-Session-Token": explicitToken } : {}),
  }
}

function createRequestOptions(
  init?: RequestInit,
  explicitToken?: string
): RequestInit & {
  baseUrl?: string
  skipAuth?: boolean
  onResponse?: (response: Response) => void
} {
  const config = getRuntimeImConfig()
  requireEntryToken(config)
  const contextKey = identityContextKey(config)
  return {
    ...init,
    skipAuth: true,
    headers: {
      ...createImHeaders(explicitToken),
      ...(init?.headers as Record<string, string> | undefined),
    },
    onResponse: (response) => {
      if (contextKey === identityContextKey(getRuntimeImConfig())) {
        applyCustomerSessionHeaders(response)
      }
    },
    baseUrl: config.baseUrl,
  }
}

function toQueryString(query?: Record<string, string | number | undefined>) {
  if (!query) {
    return ""
  }

  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === "") {
      return
    }
    params.set(key, String(value))
  })
  const output = params.toString()
  return output ? `?${output}` : ""
}

export async function exchangeCustomerSession() {
  const config = getRuntimeImConfig()
  requireEntryToken(config)
  const contextKey = identityContextKey(config)
  const result = await request<ImCustomerSessionExchangeResponse>(
    "/api/customer/session_exchange",
    {
      method: "POST",
      skipAuth: true,
      baseUrl: config.baseUrl,
      headers: createExchangeHeaders(),
    }
  )
  if (contextKey !== identityContextKey(getRuntimeImConfig())) {
    throw new Error(translateCurrentMessage("api.supportAuthenticationRequired"))
  }
  const session: ImCustomerSession = {
    ...result,
    channelId: config.channelId,
    baseUrl: config.baseUrl,
  }
  if (!isCustomerSessionValid(session) || !matchesSessionIdentity(session, config)) {
    throw new Error(translateCurrentMessage("api.supportAuthenticationRequired"))
  }
  writeCustomerSession(session)
  entryIdentityExchangeKey = contextKey
  return session
}

export async function ensureCustomerSession() {
  const config = getRuntimeImConfig()
  requireEntryToken(config)
  const cached = readCustomerSession()
  if (usableCustomerSession(cached, config)) return cached!
  return exchangeCustomerSession()
}

export function fetchImConversationDetail(id: number) {
  return request<ImConversationDetail>(`/api/conversation/${id}`, {
    ...createRequestOptions(),
  })
}

export function fetchImMessages(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<ImMessage>>(
    `/api/message/list${toQueryString(query)}`,
    createRequestOptions()
  )
}

export function createOrMatchImConversation() {
  return request<ImConversation>("/api/conversation/create_or_match", {
    ...createRequestOptions({ method: "POST" }),
  })
}

// Starts a new request instead of rejoining the unfinished one. The server caps
// how many open requests one visitor may have and answers with a localized
// message when the cap is reached.
export function createImConversation(subject?: string) {
  return request<ImConversation>("/api/conversation/create", {
    ...createRequestOptions({ method: "POST" }),
    body: JSON.stringify({ subject: subject?.trim() ?? "" }),
  })
}

// The visitor's own requests, newest activity first. Scoping comes from the
// external identity the request already carries.
export function fetchImConversationList(
  query?: Record<string, string | number | undefined>
) {
  return request<PageResult<ImConversation>>(
    `/api/conversation/list${toQueryString(query)}`,
    createRequestOptions()
  )
}

export function fetchImWidgetConfig() {
  return request<ImWidgetConfig>(
    `/api/channel/config${toQueryString({
      channelId: getRuntimeImConfig().channelId,
    })}`,
    {
      skipAuth: true,
      baseUrl: getRuntimeImConfig().baseUrl,
      headers: createChannelHeaders(),
    }
  )
}

export function closeImConversation(conversationId: number) {
  return request<void>("/api/conversation/close", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify({ conversationId }),
    }),
  })
}

export function sendImMessage(payload: {
  conversationId: number
  messageType: string
  content: string
  payload?: string
  clientMsgId?: string
}) {
  return request<ImMessage>("/api/message/send", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify(payload),
    }),
  })
}

export function markImMessageRead(conversationId: number, messageId = 0) {
  return request<void>("/api/message/read", {
    ...createRequestOptions({
      method: "POST",
      body: JSON.stringify({ conversationId, messageId }),
    }),
  })
}

export function uploadImImage(conversationId: number, file: File) {
  const formData = new FormData()
  formData.set("conversationId", String(conversationId))
  formData.set("file", file)
  return request<ImAsset>("/api/message/upload_image", {
    ...createRequestOptions({
      method: "POST",
      body: formData,
    }),
  })
}

export function uploadImAttachment(conversationId: number, file: File) {
  const formData = new FormData()
  formData.set("conversationId", String(conversationId))
  formData.set("file", file)
  return request<ImAsset>("/api/message/upload_attachment", {
    ...createRequestOptions({
      method: "POST",
      body: formData,
    }),
  })
}

export async function submitConversationSatisfaction(
  params: SubmitSatisfactionParams,
  token?: string
): Promise<ConversationSatisfactionResponse> {
  return request<ConversationSatisfactionResponse>(
    "/api/conversation/satisfaction",
    createRequestOptions(
      {
        method: "POST",
        body: JSON.stringify(params),
      },
      token
    )
  )
}

