"use client"

import { ArrowRightIcon, CircleHelpIcon } from "lucide-react"

import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

export type PromptStartersProps = {
  items?: string[] | null
  onSelect?: (text: string) => void
  disabled?: boolean
  className?: string
}

export function PromptStarters({
  items,
  onSelect,
  disabled = false,
  className,
}: PromptStartersProps) {
  const t = useI18n()
  const safeItems = Array.isArray(items)
    ? items.map((item) => item.trim()).filter(Boolean)
    : []

  if (safeItems.length === 0) {
    return null
  }

  return (
    <div
      role="group"
      aria-label={t("promptStarters.title")}
      className={cn("flex w-full max-w-md flex-col gap-2", className)}
    >
      {safeItems.map((item, index) => (
        <button
          key={`${index}-${item}`}
          type="button"
          disabled={disabled}
          onClick={() => onSelect?.(item)}
          className="group flex w-full items-center gap-2.5 rounded-lg border border-border/80 bg-card/80 px-3.5 py-2.5 text-left text-sm text-card-foreground shadow-xs transition-all hover:border-primary/40 hover:bg-accent/60 hover:text-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:scale-[0.99] disabled:pointer-events-none disabled:opacity-50 dark:bg-card/50"
        >
          <CircleHelpIcon className="size-4 shrink-0 text-muted-foreground transition-colors group-hover:text-primary" />
          <span className="flex-1 font-normal leading-snug break-words">{item}</span>
          <ArrowRightIcon className="size-3.5 shrink-0 text-muted-foreground/60 opacity-0 transition-all group-hover:opacity-100 group-hover:translate-x-0.5" />
        </button>
      ))}
    </div>
  )
}
