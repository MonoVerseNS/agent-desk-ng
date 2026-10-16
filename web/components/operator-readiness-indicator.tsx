"use client"

import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  Loader2Icon,
  RefreshCwIcon,
  ShieldAlertIcon,
  SparklesIcon,
  WrenchIcon,
} from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { useI18n } from "@/i18n/provider"
import {
  applySetupQuickFix,
  fetchSetupReadiness,
  type SetupQuickFixAction,
  type SetupReadinessResponse,
} from "@/lib/api/setup-readiness"
import { cn } from "@/lib/utils"

export const SETUP_READINESS_UPDATED_EVENT = "setup-readiness-updated"

export function notifySetupReadinessUpdated() {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(SETUP_READINESS_UPDATED_EVENT))
  }
}

export function OperatorReadinessIndicator({
  compact = false,
}: {
  compact?: boolean
}) {
  const t = useI18n()
  const [readiness, setReadiness] = useState<SetupReadinessResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [fixing, setFixing] = useState(false)
  const [open, setOpen] = useState(false)

  const loadReadiness = useCallback(async () => {
    try {
      const data = await fetchSetupReadiness()
      setReadiness(data)
    } catch {
      // Silent background check in header
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadReadiness()
    const handleUpdated = () => {
      void loadReadiness()
    }
    window.addEventListener(SETUP_READINESS_UPDATED_EVENT, handleUpdated)
    const interval = window.setInterval(() => {
      void loadReadiness()
    }, 45000)
    return () => {
      window.removeEventListener(SETUP_READINESS_UPDATED_EVENT, handleUpdated)
      window.clearInterval(interval)
    }
  }, [loadReadiness])

  async function handleQuickFix(action: SetupQuickFixAction) {
    setFixing(true)
    try {
      const res = await applySetupQuickFix({ action })
      setReadiness(res.readiness)
      notifySetupReadinessUpdated()
      toast.success(t("setupWizard.quickFixSuccess"))
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("common.operationFailed"),
      )
    } finally {
      setFixing(false)
    }
  }

  if (loading && !readiness) {
    return null
  }
  if (!readiness) {
    return null
  }

  const operatorOk = readiness.current_operator.can_receive_now
  const chainOk = readiness.can_receive_requests_now
  const isAllReady = operatorOk && chainOk
  const blockerCount =
    readiness.current_operator.blocker_codes.length +
    (chainOk ? 0 : Math.max(1, readiness.total_steps - readiness.ready_steps))

  const canQuickSetupAll = readiness.steps.some((s) =>
    s.quick_fix_actions.includes("quick_setup_all"),
  )
  const canEnableMyProfile = readiness.steps.some((s) =>
    s.quick_fix_actions.includes("enable_my_operator_profile"),
  )

  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (nextOpen) {
          void loadReadiness()
        }
      }}
    >
      <PopoverTrigger
        render={
          <Button
            variant="outline"
            size="sm"
            className={cn(
              "h-8 gap-1.5 rounded-md px-2.5 text-xs font-medium",
              isAllReady
                ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/15 dark:text-emerald-300"
                : "border-amber-500/40 bg-amber-500/10 text-amber-700 hover:bg-amber-500/15 dark:text-amber-300",
            )}
          />
        }
      >
        {isAllReady ? (
          <CheckCircle2Icon className="size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
        ) : (
          <AlertTriangleIcon className="size-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
        )}
        <span className={cn(compact && "hidden md:inline")}>
          {isAllReady
            ? t("setupWizard.headerIndicator.readyLabel")
            : t("setupWizard.headerIndicator.blockedLabel", {
                count: blockerCount,
              })}
        </span>
      </PopoverTrigger>
      <PopoverContent align="end" sideOffset={8} className="w-80 rounded-md p-3">
        <div className="space-y-3">
          <div className="flex items-start justify-between gap-2">
            <div>
              <p className="text-xs font-semibold text-foreground">
                {t("setupWizard.currentOperatorTitle")}
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">
                {t("setupWizard.stepsProgress", {
                  ready: readiness.ready_steps,
                  total: readiness.total_steps,
                })}
              </p>
            </div>
            <Button
              variant="ghost"
              size="icon"
              className="size-7 rounded-md"
              onClick={() => void loadReadiness()}
              aria-label={t("setupWizard.refresh")}
            >
              <RefreshCwIcon className="size-3.5" />
            </Button>
          </div>

          <div className="rounded-md border border-border/70 bg-muted/40 p-2.5 text-xs">
            <div className="flex flex-wrap items-center justify-between gap-1.5">
              <span className="font-medium text-foreground">
                {t("setupWizard.currentOperatorTeam", {
                  team:
                    readiness.current_operator.team_name ||
                    t("setupWizard.currentOperatorNoTeam"),
                })}
              </span>
              <Badge variant="outline" className="rounded-md text-[11px]">
                {t("setupWizard.currentOperatorLoad", {
                  active: readiness.current_operator.active_conversation_count,
                  max: readiness.current_operator.max_concurrent_count,
                })}
              </Badge>
            </div>

            {isAllReady ? (
              <p className="mt-2 flex items-center gap-1.5 text-emerald-700 dark:text-emerald-300">
                <CheckCircle2Icon className="size-3.5 shrink-0" />
                <span>{t("setupWizard.currentOperatorReady")}</span>
              </p>
            ) : (
              <div className="mt-2 space-y-1.5">
                <p className="font-medium text-amber-700 dark:text-amber-300">
                  {t("setupWizard.currentOperatorBlocked")}
                </p>
                <ul className="space-y-1 text-muted-foreground">
                  {readiness.current_operator.blocker_codes.map((code) => (
                    <li key={code} className="flex items-start gap-1.5">
                      <ShieldAlertIcon className="mt-0.5 size-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
                      <span>{t(`setupWizard.operatorBlockers.${code}`)}</span>
                    </li>
                  ))}
                  {!chainOk
                    ? readiness.steps
                        .filter((step) => step.status !== "ok")
                        .map((step) => (
                          <li key={step.key} className="flex items-start gap-1.5">
                            <AlertTriangleIcon className="mt-0.5 size-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
                            <span>
                              {t(`setupWizard.steps.${step.key}.title`)}
                            </span>
                          </li>
                        ))
                    : null}
                </ul>
              </div>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            {!operatorOk && canEnableMyProfile ? (
              <Button
                size="sm"
                className="w-full gap-1.5 rounded-md text-xs"
                disabled={fixing}
                onClick={() =>
                  void handleQuickFix(
                    chainOk || !canQuickSetupAll
                      ? "enable_my_operator_profile"
                      : "quick_setup_all",
                  )
                }
              >
                {fixing ? (
                  <Loader2Icon className="size-3.5 animate-spin" />
                ) : (
                  <SparklesIcon className="size-3.5" />
                )}
                <span>
                  {chainOk || !canQuickSetupAll
                    ? t("setupWizard.enableMyReceptionBtn")
                    : t("setupWizard.quickSetupAll")}
                </span>
              </Button>
            ) : !chainOk && canQuickSetupAll ? (
              <Button
                size="sm"
                className="w-full gap-1.5 rounded-md text-xs"
                disabled={fixing}
                onClick={() => void handleQuickFix("quick_setup_all")}
              >
                {fixing ? (
                  <Loader2Icon className="size-3.5 animate-spin" />
                ) : (
                  <SparklesIcon className="size-3.5" />
                )}
                <span>{t("setupWizard.quickSetupAll")}</span>
              </Button>
            ) : null}

            <Button
              variant="outline"
              size="sm"
              className="w-full gap-1.5 rounded-md text-xs"
              onClick={() => setOpen(false)}
              render={<Link href="/dashboard/setup" />}
            >
              <WrenchIcon className="size-3.5" />
              <span>{t("setupWizard.openSetupWizard")}</span>
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
