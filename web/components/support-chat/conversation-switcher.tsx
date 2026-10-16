"use client"

import {
  CheckCircle2Icon,
  CheckIcon,
  ChevronDownIcon,
  HeadphonesIcon,
  MessageSquarePlusIcon,
} from "lucide-react"
import { useMemo } from "react"
import { useShallow } from "zustand/react/shallow"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
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
  title: string
  subtitle: string
  status: string
  onNewRequest: () => void
  onCloseActiveRequest?: () => void
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

function getStatusDotClass(status: string) {
  if (status === "connected") {
    return "bg-emerald-500 shadow-[0_0_0_3px_rgba(16,185,129,0.14)]"
  }
  if (status === "connecting") {
    return "bg-amber-500 shadow-[0_0_0_3px_rgba(245,158,11,0.16)]"
  }
  return "bg-muted-foreground shadow-[0_0_0_3px_rgba(148,163,184,0.14)]"
}

export function SupportChatConversationSwitcher({
  title,
  subtitle,
  status,
  onNewRequest,
  onCloseActiveRequest,
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

  const activeSubject = active?.subject?.trim()
  const hasMultiple = conversations.length > 1

  const displayTitle = activeSubject
    ? activeSubject
    : hasMultiple && active
      ? resolveConversationTitle(active, t("supportChat.requestFallbackTitle"))
      : title

  const displaySubtitle = activeSubject || hasMultiple
    ? title
    : subtitle

  const avatarElement = (
    <div className="relative flex size-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary ring-1 ring-primary/15 dark:bg-primary/15 dark:ring-primary/25">
      <HeadphonesIcon className="size-4" />
      <span
        className={cn(
          "absolute -right-0.5 -bottom-0.5 size-2.5 rounded-full ring-2 ring-card",
          getStatusDotClass(status)
        )}
        aria-hidden="true"
      />
    </div>
  )

  if (!hasMultiple) {
    return (
      <div className="flex min-w-0 flex-1 items-center gap-2.5 p-1 -m-1">
        {avatarElement}
        <div className="min-w-0 flex-1 space-y-0.5">
          <div className="truncate text-sm font-semibold leading-5 text-foreground">
            {displayTitle}
          </div>
          <div className="truncate text-[11px] leading-4 text-muted-foreground">
            {displaySubtitle}
          </div>
        </div>
      </div>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            className="flex min-w-0 flex-1 items-center gap-2.5 rounded-md p-1 -m-1 text-left transition-colors hover:bg-foreground/[0.04] group focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
            aria-label={t("supportChat.myRequests")}
          >
            {avatarElement}
            <div className="min-w-0 flex-1 space-y-0.5">
              <div className="flex items-center gap-1.5">
                <span className="truncate text-sm font-semibold leading-5 text-foreground">
                  {displayTitle}
                </span>
                <ChevronDownIcon className="size-3.5 shrink-0 text-muted-foreground opacity-60 group-hover:opacity-100 transition-opacity" />
                {totalUnread > 0 ? (
                  <span className="inline-flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-none text-primary-foreground">
                    {totalUnread > 99 ? "99+" : totalUnread}
                  </span>
                ) : null}
              </div>
              <div className="truncate text-[11px] leading-4 text-muted-foreground">
                {displaySubtitle}
              </div>
            </div>
          </button>
        }
      >
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-[300px] p-0">
        <DropdownMenuGroup>
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
        </DropdownMenuGroup>
        <DropdownMenuSeparator className="my-0" />
        <div className="flex items-center gap-1.5 p-1.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={creatingConversation}
            onClick={onNewRequest}
            className="h-8 flex-1 justify-start gap-1.5 text-xs"
          >
            <MessageSquarePlusIcon className="size-3.5" />
            <span className="truncate">
              {creatingConversation
                ? t("supportChat.creatingRequest")
                : t("supportChat.newRequest")}
            </span>
          </Button>
          {active && active.status !== 4 && onCloseActiveRequest ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onCloseActiveRequest}
              className="h-8 shrink-0 gap-1 px-2 text-xs text-muted-foreground hover:bg-rose-50 hover:text-rose-600 dark:hover:bg-rose-950/45 dark:hover:text-rose-300"
              title={t("supportChat.confirmClose")}
            >
              <CheckCircle2Icon className="size-3.5" />
              <span>{t("supportChat.confirmClose")}</span>
            </Button>
          ) : null}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}