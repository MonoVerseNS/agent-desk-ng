"use client"

import { RotateCwIcon } from "lucide-react"
import { Component, type ReactNode } from "react"

import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"

function ChatCrashFallback({ onReset }: { onReset: () => void }) {
  const t = useI18n()
  return (
    <div
      role="alert"
      className="flex h-full min-h-0 w-full flex-col items-center justify-center gap-3 bg-card px-6 text-center"
    >
      <div className="text-sm font-semibold text-foreground">{t("supportChat.crashTitle")}</div>
      <p className="max-w-[260px] text-xs leading-5 text-muted-foreground">
        {t("supportChat.crashDescription")}
      </p>
      <Button type="button" size="sm" variant="outline" onClick={onReset}>
        <RotateCwIcon className="size-4" />
        {t("supportChat.crashReload")}
      </Button>
    </div>
  )
}

type State = { failed: boolean }

/**
 * Keeps a rendering failure inside the chat from blanking the whole embedded
 * frame; the visitor gets a recoverable state instead.
 */
export class SupportChatErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { failed: false }

  static getDerivedStateFromError(): State {
    return { failed: true }
  }

  componentDidCatch(error: unknown) {
    console.error("Support chat crashed", error)
  }

  render() {
    if (this.state.failed) {
      return <ChatCrashFallback onReset={() => this.setState({ failed: false })} />
    }
    return this.props.children
  }
}
