export type AgentActivityInfo = {
  conversationId: number
  stepCode: string
  toolName?: string
}

export type AgentActivityEventPayload = {
  conversationId?: number
  stepCode?: string
  toolName?: string
  status?: string
}

export function resolveAgentActivityState(
  current: AgentActivityInfo | null,
  activeConversationId: number | undefined,
  payload: AgentActivityEventPayload | null | undefined
): AgentActivityInfo | null {
  if (
    !payload ||
    !activeConversationId ||
    !payload.conversationId ||
    payload.conversationId !== activeConversationId ||
    !payload.stepCode
  ) {
    return current
  }
  if (payload.status === "completed" || payload.status === "failed") {
    return null
  }
  return {
    conversationId: payload.conversationId,
    stepCode: payload.stepCode,
    ...(payload.toolName ? { toolName: payload.toolName } : {}),
  }
}
