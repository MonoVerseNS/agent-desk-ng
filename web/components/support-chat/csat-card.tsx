"use client"

import { Loader2Icon, StarIcon } from "lucide-react"
import { useEffect, useState } from "react"

import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { useI18n } from "@/i18n/provider"
import {
  submitConversationSatisfaction,
  type ConversationSatisfactionResponse,
} from "@/lib/api/im"
import { cn } from "@/lib/utils"

export type CSATCardProps = {
  conversationId: number
  customerToken?: string
  initialSatisfaction?: ConversationSatisfactionResponse | null
  onSubmitted?: (satisfaction: ConversationSatisfactionResponse) => void
  className?: string
}

function getScoreLabel(score: number, t: (key: string) => string): string {
  switch (score) {
    case 1:
      return t("csat.score1")
    case 2:
      return t("csat.score2")
    case 3:
      return t("csat.score3")
    case 4:
      return t("csat.score4")
    case 5:
      return t("csat.score5")
    default:
      return ""
  }
}

export function CSATCard({
  conversationId,
  customerToken,
  initialSatisfaction,
  onSubmitted,
  className,
}: CSATCardProps) {
  const t = useI18n()
  const [satisfaction, setSatisfaction] = useState<ConversationSatisfactionResponse | null>(
    initialSatisfaction ?? null
  )
  const [selectedScore, setSelectedScore] = useState<number | null>(
    initialSatisfaction?.score ?? null
  )
  const [hoveredScore, setHoveredScore] = useState<number | null>(null)
  const [feedback, setFeedback] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setSatisfaction(initialSatisfaction ?? null)
    setSelectedScore(initialSatisfaction?.score ?? null)
    setHoveredScore(null)
    setFeedback("")
    setError(null)
  }, [conversationId, initialSatisfaction])

  if (satisfaction) {
    return (
      <div
        className={cn(
          "flex flex-col items-center gap-2.5 rounded-md border border-border/80 bg-card/90 p-4 text-center text-card-foreground dark:bg-card/70",
          className
        )}
      >
        <div
          className="flex items-center gap-1.5"
          role="img"
          aria-label={t("csat.alreadyRated", { score: String(satisfaction.score) })}
        >
          {[1, 2, 3, 4, 5].map((star) => (
            <StarIcon
              key={star}
              className={cn(
                "size-5",
                star <= satisfaction.score
                  ? "fill-amber-400 text-amber-400"
                  : "text-muted-foreground/25"
              )}
            />
          ))}
        </div>
        <div className="space-y-0.5">
          <p className="text-sm font-medium">{t("csat.thankYou")}</p>
          <p className="text-xs text-muted-foreground">
            {t("csat.alreadyRated", { score: String(satisfaction.score) })}
          </p>
        </div>
        {satisfaction.feedback ? (
          <p className="mt-1 max-w-xs text-xs text-muted-foreground/90 italic break-words">
            &ldquo;{satisfaction.feedback}&rdquo;
          </p>
        ) : null}
      </div>
    )
  }

  const activeScore = hoveredScore ?? selectedScore
  const activeScoreLabel = activeScore ? getScoreLabel(activeScore, t) : ""

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedScore || isSubmitting) {
      return
    }

    setIsSubmitting(true)
    setError(null)

    try {
      const result = await submitConversationSatisfaction(
        {
          conversationId,
          score: selectedScore,
          feedback: feedback.trim() || undefined,
        },
        customerToken
      )
      setSatisfaction(result)
      onSubmitted?.(result)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("api.requestFailed"))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      className={cn(
        "flex flex-col gap-3 rounded-md border border-border/80 bg-card/90 p-4 text-card-foreground dark:bg-card/70",
        className
      )}
    >
      <div className="text-center">
        <h4 className="text-sm font-medium text-foreground">{t("csat.title")}</h4>
      </div>

      <div className="flex flex-col items-center gap-1.5">
        <div
          role="radiogroup"
          aria-label={t("csat.title")}
          className="flex items-center gap-1.5"
          onMouseLeave={() => setHoveredScore(null)}
        >
          {[1, 2, 3, 4, 5].map((star) => {
            const isFilled = activeScore !== null && star <= activeScore
            const isSelected = selectedScore === star
            const starLabel = getScoreLabel(star, t)
            return (
              <button
                key={star}
                type="button"
                role="radio"
                aria-checked={isSelected}
                aria-label={`${star} - ${starLabel}`}
                disabled={isSubmitting}
                onMouseEnter={() => setHoveredScore(star)}
                onFocus={() => setHoveredScore(star)}
                onBlur={() => setHoveredScore(null)}
                onClick={() => setSelectedScore(star)}
                className="group rounded-md p-1 transition-transform hover:scale-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none"
              >
                <StarIcon
                  className={cn(
                    "size-6 transition-colors",
                    isFilled
                      ? "fill-amber-400 text-amber-400"
                      : "text-muted-foreground/35 group-hover:text-amber-400/70"
                  )}
                />
              </button>
            )
          })}
        </div>

        <div
          role="status"
          aria-live="polite"
          className="h-4 text-xs font-medium text-muted-foreground transition-all"
        >
          {activeScoreLabel}
        </div>
      </div>

      <Textarea
        value={feedback}
        onChange={(e) => setFeedback(e.target.value)}
        placeholder={t("csat.commentPlaceholder")}
        disabled={isSubmitting}
        rows={2}
        className="resize-none text-xs"
      />

      {error ? (
        <div role="alert" aria-live="polite" className="text-xs text-destructive">
          {error}
        </div>
      ) : null}

      <Button
        type="submit"
        disabled={!selectedScore || isSubmitting}
        size="sm"
        className="w-full"
      >
        {isSubmitting ? (
          <>
            <Loader2Icon className="mr-2 size-3.5 animate-spin" />
            {t("csat.submitting")}
          </>
        ) : (
          t("csat.submit")
        )}
      </Button>
    </form>
  )
}
