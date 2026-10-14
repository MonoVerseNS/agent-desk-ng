export type OperatorPresenceAction = "viewing" | "typing" | "left"

export type OperatorPresenceRecord = {
  conversationId: number
  userId: number
  username: string
  nickname: string
  action: OperatorPresenceAction
  at: number
}

export type OperatorPresencePayload = {
  conversationId?: number
  userId?: number
  username?: string
  nickname?: string
  action?: string
  at?: string | number
}

const DEFAULT_PRESENCE_TTL_MS = 15_000

export function purgeExpiredOperatorPresences(
  currentList: OperatorPresenceRecord[],
  nowMs?: number,
  maxAgeMs = DEFAULT_PRESENCE_TTL_MS
): OperatorPresenceRecord[] {
  const now = nowMs ?? Date.now()
  return (currentList || []).filter((record) => now - record.at <= maxAgeMs)
}

export function updateOperatorPresenceList(
  currentList: OperatorPresenceRecord[],
  incoming: OperatorPresencePayload,
  currentUserId: number,
  nowMs?: number
): OperatorPresenceRecord[] {
  if (!incoming || !incoming.conversationId || !incoming.userId) {
    return currentList
  }
  if (incoming.userId === currentUserId) {
    return currentList
  }

  const now = nowMs ?? Date.now()
  const list = currentList || []

  if (incoming.action === "left") {
    const nextList = list.filter(
      (item) =>
        !(item.conversationId === incoming.conversationId && item.userId === incoming.userId)
    )
    return purgeExpiredOperatorPresences(nextList, now)
  }

  if (incoming.action === "viewing" || incoming.action === "typing") {
    const existingIndex = list.findIndex(
      (item) =>
        item.conversationId === incoming.conversationId && item.userId === incoming.userId
    )
    const record: OperatorPresenceRecord = {
      conversationId: incoming.conversationId,
      userId: incoming.userId,
      username: incoming.username ?? "",
      nickname: incoming.nickname ?? "",
      action: incoming.action,
      at: now,
    }

    let nextList: OperatorPresenceRecord[]
    if (existingIndex >= 0) {
      nextList = [...list]
      nextList[existingIndex] = record
    } else {
      nextList = [...list, record]
    }
    return purgeExpiredOperatorPresences(nextList, now)
  }

  return purgeExpiredOperatorPresences(list, now)
}

export const resolveOperatorPresence = updateOperatorPresenceList
