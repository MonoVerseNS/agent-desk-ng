"use client"

import { useEffect, useRef } from "react"
import { toast } from "sonner"

import { createAdminWebSocketUrl } from "@/lib/api/admin"
import { type AgentMessage } from "@/lib/api/agent"
import { shouldReloadConversationListForRealtimePatch } from "@/lib/agent-conversation-realtime"
import { readSession } from "@/lib/auth"
import {
  normalizeRealtimeMessage,
  type RealtimeConversationPatch,
  type RealtimeMessageCreatedPayload,
} from "@/lib/im-realtime-state"
import { createRealtimeConnectionManager } from "@/lib/realtime-connection"
import { getNotificationBody, showNotification } from "@/lib/services/notification"
import { playIncomingMessageSound } from "@/lib/sound-alert"
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations"
import { useI18n } from "@/i18n/provider"

import { type RealtimeAgentActivityPayload } from "@/lib/im-realtime"
import { type OperatorPresencePayload } from "@/lib/operator-presence-state"

type AgentRealtimeConnection = ReturnType<typeof createRealtimeConnectionManager>
type AgentRealtimeEnvelope = {
  eventId?: string
  type?: string
  data?: RealtimeMessageCreatedPayload<AgentMessage> &
    RealtimeConversationPatch &
    RealtimeAgentActivityPayload &
    OperatorPresencePayload & {
      messageId?: number
      recalledAt?: string
      sendStatus?: number
    }
}

let activeRealtimeSocket: (() => WebSocket | null) | null = null

export function sendOperatorPresence(
  conversationId: number,
  action: "viewing" | "typing" | "left"
) {
  const socket = activeRealtimeSocket?.()
  if (socket && socket.readyState === WebSocket.OPEN && conversationId > 0) {
    socket.send(JSON.stringify({ type: "presence", conversationId, action }))
  }
}

export function useAgentConversationRealtime() {
  const t = useI18n()
  const selectedConversationId = useAgentConversationsStore(
    (state) => state.selectedConversationId
  )
  const setRealtimeStatus = useAgentConversationsStore(
    (state) => state.setRealtimeStatus
  )
  const realtimeRef = useRef<AgentRealtimeConnection | null>(null)
  const subscribedConversationIdRef = useRef<number | null>(null)
  const selectedConversationIdRef = useRef<number | null>(selectedConversationId)
  const currentUserIdRef = useRef<number>(readSession()?.user.id ?? 0)

  useEffect(() => {
    selectedConversationIdRef.current = selectedConversationId
  }, [selectedConversationId])

  useEffect(() => {
    const realtime = createRealtimeConnectionManager({
      createSocket: () => new WebSocket(createAdminWebSocketUrl()),
      onStatusChange: setRealtimeStatus,
      onOpen: (socket) => {
        console.info("[agent-realtime] websocket connected", {
          url: socket.url,
        })

        const conversationId = selectedConversationIdRef.current
        if (conversationId) {
          socket.send(
            JSON.stringify({
              type: "subscribe",
              topics: [`conversation:${conversationId}`],
            })
          )
          socket.send(
            JSON.stringify({
              type: "presence",
              conversationId,
              action: "viewing",
            })
          )
          subscribedConversationIdRef.current = conversationId
        } else {
          subscribedConversationIdRef.current = null
        }
      },
      onMessage: (event, socket) => {
        try {
          const envelope = JSON.parse(event.data) as AgentRealtimeEnvelope
          const eventType = envelope.type ?? ""
          const payload = envelope.data
          const conversationId = payload?.conversationId ?? 0
          const eventId = envelope.eventId?.trim() ?? ""

          if (
            eventType === "" ||
            eventType === "connected" ||
            eventType === "pong" ||
            eventType === "subscribed" ||
            eventType === "unsubscribed"
          ) {
            return
          }

          if (eventId && socket.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: "ack", eventId }))
          }

          const store = useAgentConversationsStore.getState()
          if (eventType === "resyncRequired") {
            void store.resyncRealtimeData(conversationId).catch((error) => {
              toast.error(error instanceof Error ? error.message : t("conversation.syncConversationDataFailed"))
            })
            return
          }

          if (eventType === "agent.activity" && payload) {
            store.applyRealtimeAgentActivity(payload)
            return
          }

          if (eventType === "operator.presence" && payload) {
            store.applyRealtimeOperatorPresence(payload, currentUserIdRef.current)
            return
          }

          if (eventType === "message.created") {
            const message = normalizeRealtimeMessage<AgentMessage>(payload)
            if (!message) {
              void store.resyncRealtimeData(conversationId).catch((error) => {
                toast.error(error instanceof Error ? error.message : t("conversation.syncMessagesFailed"))
              })
              return
            }
            store.applyRealtimeMessageCreated(message)

            const isCustomerMsg = message.senderType === "customer"
            const isAssignedToMe = (payload?.currentAssigneeId ?? 0) === currentUserIdRef.current
            const isUnassigned = (payload?.currentAssigneeId ?? 0) === 0
            const isCurrentActive = message.conversationId === selectedConversationIdRef.current

            if (isCustomerMsg && (isAssignedToMe || isUnassigned || isCurrentActive)) {
              playIncomingMessageSound()
              if (typeof document !== "undefined" && document.visibilityState !== "visible") {
                showNotification(t("conversation.newMessage"), getNotificationBody(message), () => {
                  void store.selectConversation(message.conversationId)
                })
              }
            }
            return
          }

          if (eventType === "message.recalled" && payload?.messageId) {
            store.applyRealtimeMessageRecalled(payload.messageId, {
              sendStatus: payload.sendStatus,
              recalledAt: payload.recalledAt,
            })
            return
          }

          if (eventType.startsWith("conversation.") && payload) {
            store.applyRealtimeConversationChanged(payload)
            if (shouldReloadConversationListForRealtimePatch(payload)) {
              void store.resyncRealtimeData(conversationId).catch((error) => {
                toast.error(error instanceof Error ? error.message : t("conversation.syncConversationListFailed"))
              })
            }
          }
        } catch {
          // ignore invalid ws payload
        }
      },
      onClose: (event, socket) => {
        console.log("[agent-realtime] websocket closed", {
          url: socket.url,
          readyState: socket.readyState,
          code: event.code,
          reason: event.reason,
          wasClean: event.wasClean,
        })
        subscribedConversationIdRef.current = null
      },
      onError: (_event, socket) => {
        console.log("[agent-realtime] websocket error", {
          url: socket.url,
          readyState: socket.readyState,
        })
      },
      onConnectError: (error) => {
        toast.error(error instanceof Error ? error.message : t("conversation.realtimeConnectFailed"))
      },
    })

    realtimeRef.current = realtime
    activeRealtimeSocket = () => realtimeRef.current?.getSocket() ?? null
    realtime.connect()

    return () => {
      activeRealtimeSocket = null
      const socket = realtimeRef.current?.getSocket()
      if (
        socket &&
        socket.readyState === WebSocket.OPEN &&
        subscribedConversationIdRef.current
      ) {
        socket.send(
          JSON.stringify({
            type: "presence",
            conversationId: subscribedConversationIdRef.current,
            action: "left",
          })
        )
      }
      realtimeRef.current = null
      realtime.disconnect()
      subscribedConversationIdRef.current = null
    }
  }, [setRealtimeStatus, t])

  useEffect(() => {
    const socket = realtimeRef.current?.getSocket()
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return
    }

    const previousConversationId = subscribedConversationIdRef.current
    const nextConversationId = selectedConversationId ?? null

    if (previousConversationId && previousConversationId !== nextConversationId) {
      socket.send(
        JSON.stringify({
          type: "presence",
          conversationId: previousConversationId,
          action: "left",
        })
      )
      socket.send(
        JSON.stringify({
          type: "unsubscribe",
          topics: [`conversation:${previousConversationId}`],
        })
      )
    }

    if (nextConversationId && nextConversationId !== previousConversationId) {
      socket.send(
        JSON.stringify({
          type: "subscribe",
          topics: [`conversation:${nextConversationId}`],
        })
      )
      socket.send(
        JSON.stringify({
          type: "presence",
          conversationId: nextConversationId,
          action: "viewing",
        })
      )
      subscribedConversationIdRef.current = nextConversationId
      return
    }

    if (!nextConversationId) {
      subscribedConversationIdRef.current = null
    }
  }, [selectedConversationId])

  useEffect(() => {
    const interval = setInterval(() => {
      useAgentConversationsStore.getState().purgeExpiredOperatorPresences()
    }, 5000)
    return () => {
      clearInterval(interval)
    }
  }, [])
}
