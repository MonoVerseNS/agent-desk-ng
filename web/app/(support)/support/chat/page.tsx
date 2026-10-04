"use client"

import { SupportPageContent, SupportPageShell } from "@/app/(support)/support/_components/support-page-shell"
import {
  isEmbeddedInHost,
  SupportChatPanel,
  SupportChatShell,
} from "@/components/support-chat/chat-shell"
import { useI18n } from "@/i18n/provider"

/**
 * This route serves two audiences. Embedded it is the widget iframe document,
 * where the host owns the size and the panel must fill the frame. Opened
 * directly it is a normal support page, so it gets the portal shell and a
 * bounded card instead of a full-viewport panel.
 */
export default function Page() {
  return isEmbeddedInHost() ? <SupportChatShell /> : <SupportChatPage />
}

function SupportChatPage() {
  const t = useI18n()

  return (
    <SupportPageShell section="home">
      <SupportPageContent className="pb-10 pt-6">
        <div className="mx-auto flex w-full max-w-4xl flex-col gap-4">
          <div className="flex flex-col gap-1">
            <h1 className="text-xl font-semibold tracking-tight text-foreground">
              {t("supportChat.pageTitle")}
            </h1>
            <p className="text-sm leading-6 text-muted-foreground">
              {t("supportChat.pageDescription")}
            </p>
          </div>
          <div className="flex h-[calc(100dvh-15rem)] min-h-[28rem] flex-col overflow-hidden rounded-md border bg-card supports-not-[height:100dvh]:h-[calc(100vh-15rem)]">
            <SupportChatPanel />
          </div>
        </div>
      </SupportPageContent>
    </SupportPageShell>
  )
}
