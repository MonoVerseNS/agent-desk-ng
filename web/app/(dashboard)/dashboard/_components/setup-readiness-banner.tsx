"use client"

import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CheckCircle2Icon,
  Loader2Icon,
  SparklesIcon,
} from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import {
  notifySetupReadinessUpdated,
  SETUP_READINESS_UPDATED_EVENT,
} from "@/components/operator-readiness-indicator"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import {
  applySetupQuickFix,
  fetchSetupReadiness,
  type SetupReadinessResponse,
} from "@/lib/api/setup-readiness"

export function SetupReadinessBanner({
  onUpdated,
}: {
  onUpdated?: () => void
}) {
  const t = useI18n()
  const [readiness, setReadiness] = useState<SetupReadinessResponse | null>(null)
  const [fixing, setFixing] = useState(false)

  const loadReadiness = useCallback(async () => {
    try {
      const data = await fetchSetupReadiness()
      setReadiness(data)
    } catch {
      // Ignore in banner if failed
    }
  }, [])

  useEffect(() => {
    void loadReadiness()
    const handleUpdated = () => {
      void loadReadiness()
    }
    window.addEventListener(SETUP_READINESS_UPDATED_EVENT, handleUpdated)
    return () => {
      window.removeEventListener(SETUP_READINESS_UPDATED_EVENT, handleUpdated)
    }
  }, [loadReadiness])

  async function handleQuickSetupAll() {
    setFixing(true)
    try {
      const res = await applySetupQuickFix({ action: "quick_setup_all" })
      setReadiness(res.readiness)
      notifySetupReadinessUpdated()
      onUpdated?.()
      toast.success(t("setupWizard.quickFixSuccess"))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("common.operationFailed"),
      )
    } finally {
      setFixing(false)
    }
  }

  if (!readiness) {
    return null
  }

  const allReady =
    readiness.can_receive_requests_now &&
    readiness.current_operator.can_receive_now &&
    readiness.overall_status === "ok"

  if (allReady) {
    return null
  }

  const canQuickSetupAll = readiness.steps.some((s) =>
    s.quick_fix_actions.includes("quick_setup_all"),
  )

  const incompleteSteps = readiness.steps.filter((s) => s.status !== "ok")

  return (
    <div className="rounded-md border border-amber-500/40 bg-amber-500/5 p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-1.5">
          <div className="flex flex-wrap items-center gap-2">
            <AlertTriangleIcon className="size-4 shrink-0 text-amber-600 dark:text-amber-400" />
            <h3 className="text-sm font-semibold text-foreground">
              {t("setupWizard.banner.title")}
            </h3>
            <Badge
              variant="outline"
              className="rounded-md border-amber-500/40 bg-amber-500/10 text-xs text-amber-700 dark:text-amber-300"
            >
              {t("setupWizard.stepsProgress", {
                ready: readiness.ready_steps,
                total: readiness.total_steps,
              })}
            </Badge>
          </div>
          <p className="text-xs text-muted-foreground">
            {t("setupWizard.banner.description")}
          </p>
          <div className="flex flex-wrap items-center gap-1.5 pt-1">
            {incompleteSteps.map((step) => (
              <Badge
                key={step.key}
                variant="secondary"
                className="rounded-md text-xs font-normal"
              >
                {step.step_number}. {t(`setupWizard.steps.${step.key}.title`)}
              </Badge>
            ))}
            {!readiness.current_operator.can_receive_now
              ? readiness.current_operator.blocker_codes.map((code) => (
                  <Badge
                    key={code}
                    variant="outline"
                    className="rounded-md border-amber-500/30 text-xs font-normal text-amber-700 dark:text-amber-300"
                  >
                    {t(`setupWizard.operatorBlockers.${code}`)}
                  </Badge>
                ))
              : null}
          </div>
        </div>

        <div className="flex shrink-0 flex-wrap items-center gap-2">
          {canQuickSetupAll ? (
            <Button
              size="sm"
              className="gap-1.5 rounded-md"
              disabled={fixing}
              onClick={() => void handleQuickSetupAll()}
            >
              {fixing ? (
                <Loader2Icon className="size-4 animate-spin" />
              ) : (
                <SparklesIcon className="size-4" />
              )}
              <span>
                {fixing
                  ? t("setupWizard.quickSetupRunning")
                  : t("setupWizard.quickSetupAll")}
              </span>
            </Button>
          ) : null}
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5 rounded-md"
            render={<Link href="/dashboard/setup" />}
          >
            <span>{t("setupWizard.openSetupWizard")}</span>
            <ArrowRightIcon className="size-4" />
          </Button>
        </div>
      </div>
    </div>
  )
}

export { CheckCircle2Icon }
