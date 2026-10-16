import type { ImConversation } from "./api/im"

/**
 * Mirrors IMConversationStatus.Closed from the generated backend enum.
 *
 * It is spelled out as a literal rather than imported because Node's type
 * stripping cannot load a TypeScript `enum`, and these helpers have to stay
 * unit-testable outside the bundler. support-conversation-list.test.mjs reads
 * the generated file and fails if this ever drifts, so this is a guarded mirror
 * of the generated enum, not a second source of truth.
 */
export const CLOSED_IM_CONVERSATION_STATUS = 4

/**
 * Parses an API timestamp string into a millisecond timestamp.
 * If the string does not specify a timezone offset, it is interpreted as UTC.
 */
export function parseApiTimestamp(
  value?: string | number | Date | null
): number {
  if (value === null || value === undefined) {
    return 0
  }
  if (value instanceof Date) {
    const time = value.getTime()
    return Number.isFinite(time) ? time : 0
  }
  if (typeof value === "number") {
    return Number.isFinite(value) ? value : 0
  }
  if (typeof value !== "string") {
    return 0
  }
  const trimmed = value.trim()
  if (!trimmed) {
    return 0
  }
  let normalized = trimmed.replace(" ", "T")
  if (!/(Z|[+-]\d{2}(?::?\d{2})?)$/i.test(normalized)) {
    normalized += "Z"
  }
  const timestamp = Date.parse(normalized)
  return Number.isFinite(timestamp) ? timestamp : 0
}

/**
 * A request counts as open until it is closed, rather than by listing the open
 * statuses. The server's own definition is "status is one of ai_serving,
 * pending, active", so any new status it introduces is open by default here
 * instead of being silently treated as finished.
 */
export function isOpenImConversation(
  conversation: ImConversation | null | undefined
): boolean {
  if (!conversation) {
    return false
  }
  return conversation.status !== CLOSED_IM_CONVERSATION_STATUS
}

function recencyTimestamp(conversation: ImConversation): number {
  const candidates = [conversation.lastActiveAt, conversation.lastMessageAt]
  for (const value of candidates) {
    if (!value) {
      continue
    }
    const parsed = parseApiTimestamp(value)
    if (parsed > 0) {
      return parsed
    }
  }
  return 0
}

/**
 * Most recently active first, matching the server's list ordering.
 *
 * A conversation that was just created has no timestamps yet, so treating an
 * unknown time as "oldest" would drop the visitor's brand new request to the
 * bottom of the switcher. Whenever either side has no usable timestamp we fall
 * back to the descending id instead: ids are monotonic, so that still puts the
 * newest request first. Array.prototype.sort is stable, so equal keys keep
 * their existing relative order instead of shuffling between renders.
 */
export function sortConversationsByRecency(
  conversations: readonly ImConversation[]
): ImConversation[] {
  return [...conversations].sort((left, right) => {
    const leftTime = recencyTimestamp(left)
    const rightTime = recencyTimestamp(right)
    if (leftTime > 0 && rightTime > 0 && leftTime !== rightTime) {
      return rightTime - leftTime
    }
    return right.id - left.id
  })
}

export function upsertConversation(
  conversations: readonly ImConversation[],
  conversation: ImConversation
): ImConversation[] {
  const existing = conversations.some((item) => item.id === conversation.id)
  const next = existing
    ? conversations.map((item) =>
        item.id === conversation.id ? { ...item, ...conversation } : item
      )
    : [conversation, ...conversations]
  return sortConversationsByRecency(next)
}

export function findConversationById(
  conversations: readonly ImConversation[],
  conversationId: number
): ImConversation | null {
  if (!conversationId) {
    return null
  }
  return conversations.find((item) => item.id === conversationId) ?? null
}

/**
 * The request to rejoin on load: the newest still-open one.
 *
 * Returns null when every request is closed, which is the signal to start a
 * fresh one. Selecting a closed request here would leave the visitor staring at
 * a read-only thread with the composer disabled and no way forward.
 */
export function pickConversationToResume(
  conversations: readonly ImConversation[]
): ImConversation | null {
  return (
    sortConversationsByRecency(conversations).find((item) =>
      isOpenImConversation(item)
    ) ?? null
  )
}

/**
 * What to show as a request's title. The visitor's own subject wins; otherwise
 * the latest message stands in, and only then do we fall back to the caller's
 * localized generic label.
 */
export function resolveConversationTitle(
  conversation: ImConversation | null | undefined,
  fallbackLabel: string
): string {
  const subject = conversation?.subject?.trim()
  if (subject) {
    return subject
  }
  const summary = conversation?.lastMessageSummary?.trim()
  if (summary) {
    return summary
  }
  return fallbackLabel
}

/**
 * Total unread across every request, for the switcher's badge. A visitor needs
 * to notice activity in a request they are not currently looking at.
 */
export function totalUnreadConversations(
  conversations: readonly ImConversation[]
): number {
  return conversations.reduce(
    (total, item) => total + (Number(item.customerUnreadCount) || 0),
    0
  )
}
/**
 * Builds the state patch for switching the visible request. `conversation` is
 * re-derived from the list together with the active id, so the header, the
 * composer and the message loader can never point at different requests.
 * Returns null when there is nothing to switch to.
 */
export function buildSelectConversationPatch(
  state: {
    conversations: readonly ImConversation[]
    activeConversationId: number
  },
  conversationId: number
) {
  const target = findConversationById(state.conversations, conversationId)
  if (!target || target.id === state.activeConversationId) {
    return null
  }
  return {
    conversations: state.conversations,
    activeConversationId: target.id,
    conversation: target,
    // Messages belong to the request they were loaded for.
    messages: [] as never[],
    messagesCursor: "",
    messagesHasMore: false,
    messagesLoadingMore: false,
    readingMessageId: 0,
    error: "",
  }
}
