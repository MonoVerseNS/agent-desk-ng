"use client"

import { useEffect, useState } from "react"
import { CopyIcon, Loader2Icon, SparklesIcon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useI18n } from "@/i18n/provider"
import { summarizeAgentConversation } from "@/lib/api/agent"

export type ConversationSummaryDialogProps = {
  open: boolean
  conversationId: number | null
  onOpenChange: (open: boolean) => void
}

export function ConversationSummaryDialog({
  open,
  conversationId,
  onOpenChange,
}: ConversationSummaryDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open ? (
        <ConversationSummaryDialogBody
          key={conversationId ? `summary-${conversationId}` : "summary"}
          conversationId={conversationId}
          onOpenChange={onOpenChange}
        />
      ) : null}
    </Dialog>
  )
}

type ConversationSummaryDialogBodyProps = {
  conversationId: number | null
  onOpenChange: (open: boolean) => void
}

function ConversationSummaryDialogBody({
  conversationId,
  onOpenChange,
}: ConversationSummaryDialogBodyProps) {
  const t = useI18n()
  const [loading, setLoading] = useState(false)
  const [summary, setSummary] = useState("")
  const [copying, setCopying] = useState(false)

  useEffect(() => {
    if (!conversationId) {
      setSummary("")
      setLoading(false)
      return
    }

    let active = true
    setLoading(true)

    summarizeAgentConversation(conversationId)
      .then((res) => {
        if (!active) return
        setSummary(res || "")
      })
      .catch((error) => {
        if (!active) return
        toast.error(error instanceof Error ? error.message : t("aiCopilot.errorGenerating"))
      })
      .finally(() => {
        if (!active) return
        setLoading(false)
      })

    return () => {
      active = false
    }
  }, [conversationId, t])

  async function handleCopy() {
    if (!summary || copying) {
      return
    }

    setCopying(true)
    try {
      await navigator.clipboard.writeText(summary)
      toast.success(t("aiCopilot.copied"))
    } catch {
      toast.error(t("aiCopilot.errorGenerating"))
    } finally {
      setCopying(false)
    }
  }

  return (
    <DialogContent className="max-w-lg">
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          <SparklesIcon className="size-4 text-primary" />
          {t("aiCopilot.summaryTitle")}
        </DialogTitle>
        <DialogDescription className="sr-only">
          {t("aiCopilot.summaryTitle")}
        </DialogDescription>
      </DialogHeader>

      <div className="min-h-[160px] max-h-[60vh] overflow-y-auto rounded-md border bg-muted/30 p-4">
        {loading ? (
          <div className="flex h-36 flex-col items-center justify-center gap-2 text-muted-foreground">
            <Loader2Icon className="size-5 animate-spin text-primary" />
            <span className="text-sm">{t("aiCopilot.summarizing")}</span>
          </div>
        ) : summary ? (
          <div className="whitespace-pre-wrap text-sm leading-6 select-text text-foreground">
            {summary}
          </div>
        ) : (
          <div className="flex h-36 items-center justify-center text-sm text-muted-foreground">
            {t("aiCopilot.noSummary")}
          </div>
        )}
      </div>

      <DialogFooter>
        <Button
          type="button"
          variant="outline"
          onClick={() => void handleCopy()}
          disabled={loading || !summary || copying}
        >
          <CopyIcon className="size-4" />
          {copying ? t("aiCopilot.copied") : t("aiCopilot.copy")}
        </Button>
        <Button type="button" onClick={() => onOpenChange(false)}>
          {t("common.close")}
        </Button>
      </DialogFooter>
    </DialogContent>
  )
}
