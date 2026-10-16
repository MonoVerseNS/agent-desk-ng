"use client"

import {
  AlertTriangleIcon,
  ArrowRightIcon,
  CheckCircle2Icon,
  ExternalLinkIcon,
  Loader2Icon,
  PlayIcon,
  RefreshCwIcon,
  ShieldAlertIcon,
  SparklesIcon,
  UserCheckIcon,
  XCircleIcon,
} from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useMemo, useState } from "react"
import { toast } from "sonner"

import { DashboardPage } from "@/components/dashboard-page"
import {
  notifySetupReadinessUpdated,
  SETUP_READINESS_UPDATED_EVENT,
} from "@/components/operator-readiness-indicator"
import { OptionCombobox } from "@/components/option-combobox"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useI18n } from "@/i18n/provider"
import {
  applySetupQuickFix,
  fetchSetupReadiness,
  simulateSetupRoute,
  type SetupQuickFixAction,
  type SetupReadinessResponse,
  type SetupSimulateRouteResponse,
  type SetupStepStatus,
} from "@/lib/api/setup-readiness"
import { cn } from "@/lib/utils"

function StatusIcon({ status }: { status: SetupStepStatus }) {
  if (status === "ok") {
    return (
      <CheckCircle2Icon className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
    )
  }
  if (status === "warning") {
    return (
      <AlertTriangleIcon className="size-4 shrink-0 text-amber-600 dark:text-amber-400" />
    )
  }
  return (
    <XCircleIcon className="size-4 shrink-0 text-destructive" />
  )
}

function StatusBadge({ status }: { status: SetupStepStatus }) {
  const t = useI18n()
  if (status === "ok") {
    return (
      <Badge
        variant="outline"
        className="rounded-md border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
      >
        {t("setupWizard.stepBadgeOk")}
      </Badge>
    )
  }
  if (status === "warning") {
    return (
      <Badge
        variant="outline"
        className="rounded-md border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300"
      >
        {t("setupWizard.stepBadgeWarning")}
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className="rounded-md border-destructive/40 bg-destructive/10 text-destructive"
    >
      {t("setupWizard.stepBadgeError")}
    </Badge>
  )
}

export function SetupWizardView() {
  const t = useI18n()
  const [readiness, setReadiness] = useState<SetupReadinessResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [fixingAction, setFixingAction] = useState<string | null>(null)

  const [selectedChannelId, setSelectedChannelId] = useState<string>("")
  const [simulation, setSimulation] =
    useState<SetupSimulateRouteResponse | null>(null)
  const [simulating, setSimulating] = useState(false)

  const runSimulation = useCallback(async (channelIdNum?: number) => {
    setSimulating(true)
    try {
      const sim = await simulateSetupRoute({
        channel_id: channelIdNum && channelIdNum > 0 ? channelIdNum : undefined,
        bypass_ai: true,
      })
      setSimulation(sim)
      if (sim.channel_id > 0) {
        setSelectedChannelId(String(sim.channel_id))
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("common.operationFailed"),
      )
    } finally {
      setSimulating(false)
    }
  }, [t])

  const loadAll = useCallback(async () => {
    setLoading(true)
    try {
      const data = await fetchSetupReadiness()
      setReadiness(data)
      const currentNum = Number(selectedChannelId)
      const targetChannelId =
        currentNum > 0
          ? currentNum
          : data.channels.find((ch) => ch.status === 0)?.id ??
            data.channels[0]?.id ??
            0
      if (targetChannelId > 0) {
        setSelectedChannelId(String(targetChannelId))
      }
      await runSimulation(targetChannelId)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("common.loadFailed"),
      )
    } finally {
      setLoading(false)
    }
  }, [runSimulation, selectedChannelId, t])

  useEffect(() => {
    void loadAll()
    const handleUpdated = () => {
      void loadAll()
    }
    window.addEventListener(SETUP_READINESS_UPDATED_EVENT, handleUpdated)
    return () => {
      window.removeEventListener(SETUP_READINESS_UPDATED_EVENT, handleUpdated)
    }
  }, [loadAll])

  async function handleQuickFix(
    action: SetupQuickFixAction,
    entityId?: number,
  ) {
    const key = entityId ? `${action}:${entityId}` : action
    setFixingAction(key)
    try {
      const res = await applySetupQuickFix({ action, entity_id: entityId })
      setReadiness(res.readiness)
      notifySetupReadinessUpdated()
      toast.success(t("setupWizard.quickFixSuccess"))
      const targetChannelId =
        Number(selectedChannelId) ||
        res.readiness.channels.find((ch) => ch.status === 0)?.id ||
        0
      await runSimulation(targetChannelId)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t("common.operationFailed"),
      )
    } finally {
      setFixingAction(null)
    }
  }

  const canQuickSetupAll = useMemo(() => {
    return (
      readiness?.steps.some((s) =>
        s.quick_fix_actions.includes("quick_setup_all"),
      ) ?? false
    )
  }, [readiness])

  const canEnableMyProfile = useMemo(() => {
    return (
      readiness?.steps.some((s) =>
        s.quick_fix_actions.includes("enable_my_operator_profile"),
      ) ?? false
    )
  }, [readiness])

  const channelOptions = useMemo(() => {
    if (!readiness) {
      return []
    }
    return readiness.channels.map((ch) => ({
      value: String(ch.id),
      label: `${ch.name} (${ch.channel_type})`,
    }))
  }, [readiness])

  if (loading && !readiness) {
    return (
      <DashboardPage>
        <div className="flex min-h-64 items-center justify-center rounded-md border border-border/70 bg-card p-8">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2Icon className="size-4 animate-spin" />
            <span>{t("common.loadingData")}</span>
          </div>
        </div>
      </DashboardPage>
    )
  }

  if (!readiness) {
    return null
  }

  const progressPercent = Math.round(
    (readiness.ready_steps / Math.max(readiness.total_steps, 1)) * 100,
  )

  return (
    <DashboardPage className="space-y-5">
      {/* Top Overview Header */}
      <div className="rounded-md border border-border/70 bg-card p-4 lg:p-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-1.5">
            <div className="flex flex-wrap items-center gap-2.5">
              <h1 className="text-lg font-semibold text-foreground">
                {t("setupWizard.title")}
              </h1>
              <Badge
                variant="outline"
                className={cn(
                  "rounded-md text-xs",
                  readiness.can_receive_requests_now
                    ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                    : "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300",
                )}
              >
                {readiness.can_receive_requests_now
                  ? t("setupWizard.statusReady")
                  : t("setupWizard.statusNeedsSetup", {
                      ready: readiness.ready_steps,
                      total: readiness.total_steps,
                    })}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground">
              {t("setupWizard.description")}
            </p>
          </div>

          <div className="flex shrink-0 flex-wrap items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5 rounded-md"
              disabled={loading || fixingAction !== null}
              onClick={() => void loadAll()}
            >
              <RefreshCwIcon
                className={cn("size-4", loading && "animate-spin")}
              />
              <span>{t("setupWizard.refresh")}</span>
            </Button>
            {canQuickSetupAll ? (
              <Button
                size="sm"
                className="gap-1.5 rounded-md"
                disabled={fixingAction !== null}
                onClick={() => void handleQuickFix("quick_setup_all")}
              >
                {fixingAction === "quick_setup_all" ? (
                  <Loader2Icon className="size-4 animate-spin" />
                ) : (
                  <SparklesIcon className="size-4" />
                )}
                <span>
                  {fixingAction === "quick_setup_all"
                    ? t("setupWizard.quickSetupRunning")
                    : t("setupWizard.quickSetupAll")}
                </span>
              </Button>
            ) : null}
          </div>
        </div>

        {/* Progress Bar */}
        <div className="mt-4 space-y-1.5">
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>
              {t("setupWizard.stepsProgress", {
                ready: readiness.ready_steps,
                total: readiness.total_steps,
              })}
            </span>
            <span>{progressPercent}%</span>
          </div>
          <div className="h-2 w-full overflow-hidden rounded-md bg-muted">
            <div
              className={cn(
                "h-full transition-all duration-300",
                readiness.can_receive_requests_now
                  ? "bg-emerald-500"
                  : "bg-amber-500",
              )}
              style={{ width: `${progressPercent}%` }}
            />
          </div>
        </div>
      </div>

      {/* Current Operator Personal Diagnostic Card */}
      <div
        className={cn(
          "rounded-md border p-4",
          readiness.current_operator.can_receive_now
            ? "border-emerald-500/30 bg-emerald-500/5"
            : "border-amber-500/40 bg-amber-500/5",
        )}
      >
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-1.5">
            <div className="flex flex-wrap items-center gap-2">
              <UserCheckIcon
                className={cn(
                  "size-4 shrink-0",
                  readiness.current_operator.can_receive_now
                    ? "text-emerald-600 dark:text-emerald-400"
                    : "text-amber-600 dark:text-amber-400",
                )}
              />
              <h2 className="text-sm font-semibold text-foreground">
                {t("setupWizard.currentOperatorTitle")}:{" "}
                {readiness.current_operator.nickname ||
                  readiness.current_operator.username}
              </h2>
              <Badge variant="outline" className="rounded-md text-xs">
                {t("setupWizard.currentOperatorTeam", {
                  team:
                    readiness.current_operator.team_name ||
                    t("setupWizard.currentOperatorNoTeam"),
                })}
              </Badge>
              <Badge variant="outline" className="rounded-md text-xs">
                {t("setupWizard.currentOperatorLoad", {
                  active: readiness.current_operator.active_conversation_count,
                  max: readiness.current_operator.max_concurrent_count,
                })}
              </Badge>
            </div>

            {readiness.current_operator.can_receive_now ? (
              <p className="text-xs text-emerald-700 dark:text-emerald-300">
                {t("setupWizard.currentOperatorReady")}
              </p>
            ) : (
              <div className="space-y-1">
                <p className="text-xs font-medium text-amber-700 dark:text-amber-300">
                  {t("setupWizard.currentOperatorBlocked")}
                </p>
                <div className="flex flex-wrap gap-1.5 pt-0.5">
                  {readiness.current_operator.blocker_codes.map((code) => (
                    <Badge
                      key={code}
                      variant="secondary"
                      className="gap-1 rounded-md text-xs font-normal"
                    >
                      <ShieldAlertIcon className="size-3 text-amber-600 dark:text-amber-400" />
                      <span>{t(`setupWizard.operatorBlockers.${code}`)}</span>
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </div>

          {!readiness.current_operator.can_receive_now && canEnableMyProfile ? (
            <div className="flex shrink-0 items-center gap-2">
              <Button
                size="sm"
                className="gap-1.5 rounded-md"
                disabled={fixingAction !== null}
                onClick={() =>
                  void handleQuickFix("enable_my_operator_profile")
                }
              >
                {fixingAction === "enable_my_operator_profile" ? (
                  <Loader2Icon className="size-4 animate-spin" />
                ) : (
                  <SparklesIcon className="size-4" />
                )}
                <span>{t("setupWizard.enableMyReceptionBtn")}</span>
              </Button>
            </div>
          ) : null}
        </div>
      </div>

      {/* 6-Step Checklist Grid */}
      <div className="space-y-3">
        <h2 className="text-base font-semibold text-foreground">
          {t("setupWizard.checklistSectionTitle")}
        </h2>
        <div className="grid gap-3 md:grid-cols-2">
          {readiness.steps.map((step) => {
            const primaryQuickFix = step.quick_fix_actions[0]
            return (
              <div
                key={step.key}
                className="flex flex-col justify-between rounded-md border border-border/70 bg-card p-4"
              >
                <div className="space-y-2.5">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <StatusIcon status={step.status} />
                      <h3 className="text-sm font-semibold text-foreground">
                        {step.step_number}.{" "}
                        {t(`setupWizard.steps.${step.key}.title`)}
                      </h3>
                    </div>
                    <div className="flex shrink-0 items-center gap-1.5">
                      <Badge
                        variant="secondary"
                        className="rounded-md text-xs font-normal"
                      >
                        {t("setupWizard.readyCountLabel", {
                          ready: step.ready_count,
                          total: step.total_count,
                        })}
                      </Badge>
                      <StatusBadge status={step.status} />
                    </div>
                  </div>

                  <p className="text-xs text-muted-foreground">
                    {t(`setupWizard.steps.${step.key}.description`)}
                  </p>

                  {step.issues.length > 0 ? (
                    <div className="space-y-1.5 rounded-md border border-border/60 bg-muted/30 p-2.5">
                      {step.issues.map((issue, idx) => (
                        <div
                          key={`${issue.code}-${issue.entity_id}-${idx}`}
                          className="flex items-start justify-between gap-2 text-xs"
                        >
                          <div className="flex items-start gap-1.5 text-foreground">
                            <AlertTriangleIcon className="mt-0.5 size-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
                            <span>
                              {issue.entity_name ? (
                                <strong className="font-medium">
                                  [{issue.entity_name}]{" "}
                                </strong>
                              ) : null}
                              {t(`setupWizard.issues.${issue.code}`)}
                            </span>
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>

                <div className="mt-4 flex flex-wrap items-center justify-end gap-2 border-t border-border/50 pt-3">
                  {step.key !== "end_to_end" ? (
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-8 gap-1.5 rounded-md text-xs"
                      render={
                        <Link
                          href={
                            step.key === "schedules"
                              ? "/dashboard/agent-team-schedules"
                              : step.key === "ai_agents"
                                ? "/dashboard/ai-agents"
                                : step.key === "channels"
                                  ? "/dashboard/channels"
                                  : "/dashboard/agents"
                          }
                        />
                      }
                    >
                      <span>{t("setupWizard.openSettings")}</span>
                      <ExternalLinkIcon className="size-3.5" />
                    </Button>
                  ) : null}

                  {primaryQuickFix ? (
                    <Button
                      size="sm"
                      className="h-8 gap-1.5 rounded-md text-xs"
                      disabled={fixingAction !== null}
                      onClick={() => void handleQuickFix(primaryQuickFix)}
                    >
                      {fixingAction === primaryQuickFix ? (
                        <Loader2Icon className="size-3.5 animate-spin" />
                      ) : (
                        <SparklesIcon className="size-3.5" />
                      )}
                      <span>
                        {t(`setupWizard.quickFixLabels.${primaryQuickFix}`)}
                      </span>
                    </Button>
                  ) : null}
                </div>
              </div>
            )
          })}
        </div>
      </div>

      {/* Interactive Route Simulator */}
      <div className="space-y-4 rounded-md border border-border/70 bg-card p-4 lg:p-5">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-1">
            <h2 className="text-base font-semibold text-foreground">
              {t("setupWizard.simulator.title")}
            </h2>
            <p className="text-xs text-muted-foreground">
              {t("setupWizard.simulator.description")}
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {channelOptions.length > 0 ? (
              <OptionCombobox
                options={channelOptions}
                value={selectedChannelId}
                onChange={(val) => {
                  setSelectedChannelId(val)
                  void runSimulation(Number(val))
                }}
                placeholder={t("setupWizard.simulator.selectChannelPlaceholder")}
                triggerClassName="w-64 rounded-md"
              />
            ) : null}
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5 rounded-md"
              disabled={simulating}
              onClick={() => void runSimulation(Number(selectedChannelId))}
            >
              {simulating ? (
                <Loader2Icon className="size-4 animate-spin" />
              ) : (
                <PlayIcon className="size-4" />
              )}
              <span>
                {simulating
                  ? t("setupWizard.simulator.running")
                  : t("setupWizard.simulator.runSimulation")}
              </span>
            </Button>
          </div>
        </div>

        {simulation ? (
          <div className="space-y-4">
            {/* Outcome Banner */}
            <div
              className={cn(
                "flex flex-col gap-2 rounded-md border p-3 sm:flex-row sm:items-center sm:justify-between",
                simulation.will_reach_operator
                  ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-800 dark:text-emerald-200"
                  : "border-destructive/40 bg-destructive/10 text-destructive",
              )}
            >
              <div className="flex items-center gap-2 text-sm font-medium">
                {simulation.will_reach_operator ? (
                  <CheckCircle2Icon className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                ) : (
                  <XCircleIcon className="size-4 shrink-0 text-destructive" />
                )}
                <span>
                  {simulation.will_reach_operator
                    ? t("setupWizard.simulator.outcomeSuccess", {
                        operator:
                          simulation.matched_operator?.display_name ?? "",
                      })
                    : t("setupWizard.simulator.outcomeBlocked")}
                </span>
              </div>
              <span className="text-xs opacity-90">
                {t(
                  `setupWizard.simulator.outcomes.${simulation.final_outcome_code}`,
                )}
              </span>
            </div>

            {/* 5-Hop Visual Trace Pipeline */}
            <div className="grid gap-2.5 lg:grid-cols-5">
              {simulation.hops.map((hop, idx) => (
                <div
                  key={`${hop.hop_key}-${idx}`}
                  className={cn(
                    "flex flex-col justify-between rounded-md border p-3 text-xs",
                    hop.status === "ok"
                      ? "border-emerald-500/30 bg-emerald-500/5"
                      : hop.status === "warning"
                        ? "border-amber-500/40 bg-amber-500/5"
                        : "border-destructive/40 bg-destructive/5",
                  )}
                >
                  <div className="space-y-1.5">
                    <div className="flex items-center justify-between gap-1.5">
                      <span className="font-semibold text-foreground">
                        {idx + 1}.{" "}
                        {t(`setupWizard.simulator.hops.${hop.hop_key}`)}
                      </span>
                      <StatusIcon status={hop.status} />
                    </div>
                    {hop.entity_name ? (
                      <p className="truncate font-medium text-foreground">
                        {hop.entity_name}
                      </p>
                    ) : null}
                    <p className="text-muted-foreground">
                      {t(`setupWizard.simulator.hopReasons.${hop.reason_code}`)}
                    </p>
                  </div>

                  {hop.quick_fix_action && hop.status !== "ok" ? (
                    <Button
                      size="sm"
                      variant="outline"
                      className="mt-2.5 h-7 w-full gap-1 rounded-md text-[11px]"
                      disabled={fixingAction !== null}
                      onClick={() =>
                        void handleQuickFix(
                          hop.quick_fix_action!,
                          hop.entity_id,
                        )
                      }
                    >
                      <SparklesIcon className="size-3" />
                      <span className="truncate">
                        {t(
                          `setupWizard.quickFixLabels.${hop.quick_fix_action}`,
                        )}
                      </span>
                    </Button>
                  ) : null}
                </div>
              ))}
            </div>

            {/* Candidate Operators Evaluation Table */}
            {simulation.candidate_operators.length > 0 ? (
              <div className="space-y-2">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t("setupWizard.simulator.candidateTableTitle")}
                </h3>
                <div className="overflow-hidden rounded-md border border-border/70">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>
                          {t("setupWizard.simulator.colOperator")}
                        </TableHead>
                        <TableHead>
                          {t("setupWizard.simulator.colTeam")}
                        </TableHead>
                        <TableHead>
                          {t("setupWizard.simulator.colStatus")}
                        </TableHead>
                        <TableHead>
                          {t("setupWizard.simulator.colAutoAssign")}
                        </TableHead>
                        <TableHead>
                          {t("setupWizard.simulator.colLoad")}
                        </TableHead>
                        <TableHead>
                          {t("setupWizard.simulator.colResult")}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {simulation.candidate_operators.map((cand) => (
                        <TableRow key={cand.user_id}>
                          <TableCell className="font-medium">
                            {cand.display_name}
                          </TableCell>
                          <TableCell>
                            {cand.team_name ||
                              t("setupWizard.currentOperatorNoTeam")}
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant="outline"
                              className="rounded-md text-xs"
                            >
                              {cand.service_status === 0
                                ? t("setupWizard.simulator.statusIdle")
                                : t("setupWizard.simulator.statusBusy")}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {cand.auto_assign_enabled
                              ? t("common.enabled")
                              : t("common.disabled")}
                          </TableCell>
                          <TableCell>
                            {cand.active_count} / {cand.max_concurrent_count}
                          </TableCell>
                          <TableCell>
                            {cand.eligible ? (
                              <Badge
                                variant="outline"
                                className="rounded-md border-emerald-500/30 bg-emerald-500/10 text-xs text-emerald-700 dark:text-emerald-300"
                              >
                                {t("setupWizard.simulator.eligibleBadge")}
                              </Badge>
                            ) : (
                              <span className="text-xs text-amber-700 dark:text-amber-300">
                                {t(
                                  `setupWizard.simulator.skipReasons.${cand.skip_reason_code}`,
                                )}
                              </span>
                            )}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </DashboardPage>
  )
}

export { ArrowRightIcon }
