import { translateCurrentMessage, translateMessage } from "@/i18n/messages"
import type { AppLocale } from "@/i18n/config"

/**
 * Localized display names for IMConversationStatus.
 *
 * The generated enum labels are Chinese-only, so they cannot be shown to an
 * English or Russian visitor. Keeping the number-to-key mapping here - rather
 * than switching on locale inside a component - means the widget, the demo and
 * any later surface all read the same vocabulary.
 */
const CONVERSATION_STATUS_MESSAGE_KEYS: Record<number, string> = {
  1: "supportChat.statusAiServing",
  2: "supportChat.statusPending",
  3: "supportChat.statusActive",
  4: "supportChat.statusClosed",
}

/**
 * Falls back to the closed label for an unknown status. A request is either
 * finished or not, and mislabelling it as finished is the safer error: it
 * invites the visitor to reopen it rather than hiding that it is still live.
 */
const UNKNOWN_STATUS_MESSAGE_KEY = "supportChat.statusClosed"

export function getConversationStatusMessageKey(status: number): string {
  return CONVERSATION_STATUS_MESSAGE_KEYS[status] ?? UNKNOWN_STATUS_MESSAGE_KEY
}

export function getConversationStatusLabel(
  status: number,
  locale?: AppLocale
): string {
  const key = getConversationStatusMessageKey(status)
  return locale ? translateMessage(locale, key) : translateCurrentMessage(key)
}

/**
 * The generic title for a request the visitor did not name. Subject wins when
 * present; this is what a list row shows instead.
 */
export function getConversationFallbackTitle(locale?: AppLocale): string {
  return locale
    ? translateMessage(locale, "supportChat.requestFallbackTitle")
    : translateCurrentMessage("supportChat.requestFallbackTitle")
}