"use client"

import { CheckIcon, ChevronDownIcon, MessageSquarePlusIcon } from "lucide-react"
import { useMemo } from "react"
import { useShallow } from "zustand/react/shallow"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import type { ImConversation } from "@/lib/api/im"
import { getConversationStatusMessageKey } from "@/lib/support-conversation-i18n"
import {
  isOpenImConversation,
  resolveConversationTitle,
  totalUnreadConversations,
} from "@/lib/support-conversation-list"
import { useSupportChatStore } from "@/lib/stores/support-chat"
import { cn, formatDateTime } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

type SupportChatConversationSwitcherProps = {
  onNewRequest: () => void
}

function getStatusBadgeClass(status: number) {
  if (status === 4) {
    return "bg-muted text-muted-foreground"
  }
  if (status === 2) {
    return "bg-amber-500/12 text-amber-700 dark:bg-amber-400/15 dark:text-amber-300"
  }
  return "bg-emerald-500/12 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-300"
}

export function SupportChatConversationSwitcher({
  onNewRequest,
}: SupportChatConversationSwitcherProps) {
  const t = useI18n()

  const {
    conversations,
    activeConversationId,
    creatingConversation,
    selectConversation,
  } = useSupportChatStore(
    useShallow((state) => ({
      conversations: state.conversations,
      activeConversationId: state.activeConversationId,
      creatingConversation: state.creatingConversation,
      selectConversation: state.selectConversation,
    }))
  )

  const openCount = useMemo(
    () => conversations.filter((item) => isOpenImConversation(item)).length,
    [conversations]
  )
  const totalUnread = useMemo(
    () => totalUnreadConversations(conversations),
    [conversations]
  )
  const active = useMemo(
    () => conversations.find((item) => item.id === activeConversationId) ?? null,
    [conversations, activeConversationId]
  )

  // A visitor with a single open request has nothing to switch between, so the
  // control would be a dead end. Keep it for the new-request action only.
  if (conversations.length <= 1) {
    return (
      <Button
        type="button"
        variant="ghost"
        size="sm"
        disabled={creatingConversation}
        onClick={onNewRequest}
        className="h-8 shrink-0 gap-1.5 px-2 text-xs font-medium text-muted-foreground hover:text-foreground"
      >
        <MessageSquarePlusIcon className="size-4" />
        <span className="hidden sm:inline">
          {creatingConversation ? t("supportChat.creatingRequest") : t("supportChat.newRequest")}
        </span>
      </Button>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-8 min-w-0 max-w-[190px] gap-1.5 px-2 text-xs font-medium text-muted-foreground hover:text-foreground"
          >
            <span className="truncate">
              {resolveConversationTitle(active, t("supportChat.requestFallbackTitle"))}
            </span>
            {totalUnread > 0 ? (
              <span className="inline-flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-none text-primary-foreground">
                {totalUnread > 99 ? "99+" : totalUnread}
              </span>
            ) : null}
            <ChevronDownIcon className="size-3.5 shrink-0 opacity-60" />
          </Button>
        }
      >
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[300px] p-0">
        <DropdownMenuLabel className="flex items-center justify-between gap-2 px-3 py-2 text-xs font-medium text-muted-foreground">
          <span>{t("supportChat.myRequests")}</span>
          <span className="shrink-0 font-normal">
            {t("supportChat.openRequestCount", {
              open: openCount,
              total: conversations.length,
            })}
          </span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator className="my-0" />
        <div className="max-h-64 overflow-y-auto py-1">
          {conversations.map((conversation: ImConversation) => (
            <DropdownMenuItem
              key={conversation.id}
              onClick={() => void selectConversation(conversation.id)}
              className="flex-col items-start gap-1 px-3 py-2"
            >
              <span className="flex w-full min-w-0 items-center gap-1.5">
                <CheckIcon
                  className={cn(
                    "size-3.5 shrink-0 text-primary",
                    conversation.id === activeConversationId
                      ? "opacity-100"
                      : "opacity-0"
                  )}
                  aria-hidden="true"
                />
                <span className="min-w-0 flex-1 truncate text-xs font-medium text-foreground">
                  {resolveConversationTitle(
                    conversation,
                    t("supportChat.requestFallbackTitle")
                  )}
                </span>
                {Number(conversation.customerUnreadCount) > 0 ? (
                  <span className="inline-flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-none text-primary-foreground">
                    {conversation.customerUnreadCount}
                  </span>
                ) : null}
              </span>
              <span className="flex w-full items-center gap-1.5 pl-5 text-[11px] text-muted-foreground">
                <span
                  className={cn(
                    "inline-flex shrink-0 items-center rounded-full px-1.5 py-0.5 text-[10px] font-medium",
                    getStatusBadgeClass(conversation.status)
                  )}
                >
                  {t(getConversationStatusMessageKey(conversation.status))}
                </span>
                <span className="truncate">
                  {formatDateTime(conversation.lastActiveAt)}
                </span>
              </span>
            </DropdownMenuItem>
          ))}
        </div>
        <DropdownMenuSeparator className="my-0" />
        <div className="p-1.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={creatingConversation}
            onClick={onNewRequest}
            className="h-8 w-full justify-start gap-1.5 text-xs"
          >
            <MessageSquarePlusIcon className="size-3.5" />
            {creatingConversation
              ? t("supportChat.creatingRequest")
              : t("supportChat.newRequest")}
          </Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}