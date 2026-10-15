"use client"

import { memo } from "react"
import { BotIcon, Loader2Icon } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { ConversationMessageRow } from "@/components/chat/conversation-message-row"
import { ConversationMessageBubble } from "@/components/chat/conversation-message-bubble"
import { useI18n } from "@/i18n/provider"
import { getAgentActivityLabel } from "@/lib/agent-activity-label"
import { cn } from "@/lib/utils"

export type AgentActivityBubbleProps = {
  stepCode: string
  toolName?: string
  className?: string
  isCustomer?: boolean
}

export const AgentActivityBubble = memo(function AgentActivityBubble({
  stepCode,
  toolName,
  className,
  isCustomer = true,
}: AgentActivityBubbleProps) {
  const t = useI18n()

  const label = getAgentActivityLabel({ stepCode, toolName, isCustomer }, t)

  return (
    <ConversationMessageRow
      align="start"
      avatar={
        <Avatar className="size-8">
          <AvatarFallback className="bg-primary/10 text-primary">
            <BotIcon aria-hidden="true" className="size-4" />
          </AvatarFallback>
        </Avatar>
      }
      contentClassName="max-w-[86%] gap-1.5"
      className={cn("my-1 animate-in fade-in-50 duration-200", className)}
    >
      <ConversationMessageBubble
        variant="system"
        role="status"
        aria-live="polite"
        className="flex items-center gap-2 rounded-lg border border-solid border-border/60 bg-muted/50 px-3 py-2 text-xs text-muted-foreground shadow-sm"
      >
        <Loader2Icon aria-hidden="true" className="size-3.5 shrink-0 animate-spin text-primary" />
        <span className="font-medium text-foreground/80">{label}</span>
      </ConversationMessageBubble>
    </ConversationMessageRow>
  )
})
