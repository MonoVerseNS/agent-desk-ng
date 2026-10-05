"use client"

import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  fetchDashboardBranding,
  saveDashboardBranding,
  type DashboardBranding,
} from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"

/**
 * Renames the deployment.
 *
 * The name applies to the visitor portal, the dashboard chrome and the widget,
 * because all three read the same resolved branding. Clearing a field hands
 * control back to configuration rather than blanking the name, so a deployment
 * that sets COMPANY_NAME in its environment never ends up nameless.
 */
export default function DashboardSettingsPage() {
  const t = useI18n()
  const [branding, setBranding] = useState<DashboardBranding | null>(null)
  const [companyName, setCompanyName] = useState("")
  const [companyLogoUrl, setCompanyLogoUrl] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [loadError, setLoadError] = useState("")

  const load = useCallback(async () => {
    setLoading(true)
    setLoadError("")
    try {
      const result = await fetchDashboardBranding()
      setBranding(result)
      setCompanyName(result.companyName ?? "")
      setCompanyLogoUrl(result.companyLogoUrl ?? "")
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : t("settings.brandingLoadFailed"))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void load()
  }, [load])

  async function handleSave() {
    if (saving) {
      return
    }
    setSaving(true)
    try {
      const result = await saveDashboardBranding({
        companyName,
        companyLogoUrl,
      })
      setBranding(result)
      setCompanyName(result.companyName ?? "")
      setCompanyLogoUrl(result.companyLogoUrl ?? "")
      toast.success(t("settings.brandingSaved"))
      // The name appears in the portal header too, which caches the public config
      // for the session, so a reload is the honest way to show the new value.
      window.location.reload()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("settings.brandingSaveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto w-full max-w-3xl space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>{t("settings.brandingTitle")}</CardTitle>
          <CardDescription>{t("settings.brandingDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          {loadError ? (
            <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
              {loadError}
            </div>
          ) : null}

          <Field>
            <FieldLabel htmlFor="branding-company-name">
              {t("systemConfig.branding.companyName.title")}
            </FieldLabel>
            <Input
              id="branding-company-name"
              value={companyName}
              maxLength={120}
              disabled={loading || saving}
              placeholder={t("app.brand")}
              onChange={(event) => setCompanyName(event.target.value)}
            />
            <FieldDescription>
              {t("systemConfig.branding.companyName.description")}
            </FieldDescription>
          </Field>

          <Field>
            <FieldLabel htmlFor="branding-company-logo">
              {t("systemConfig.branding.companyLogoUrl.title")}
            </FieldLabel>
            <Input
              id="branding-company-logo"
              value={companyLogoUrl}
              maxLength={500}
              disabled={loading || saving}
              placeholder="/images/logo.svg"
              onChange={(event) => setCompanyLogoUrl(event.target.value)}
            />
            <FieldDescription>
              {t("systemConfig.branding.companyLogoUrl.description")}
            </FieldDescription>
          </Field>

          {branding?.usesConfiguredFallback ? (
            <p className="rounded-md border border-border/70 bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
              {t("settings.brandingConfiguredFallback", {
                name: branding.configuredCompanyName,
              })}
            </p>
          ) : null}
        </CardContent>
        <CardFooter className="gap-2">
          <Button type="button" disabled={loading || saving} onClick={() => void handleSave()}>
            {saving ? t("common.saving") : t("common.save")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={loading || saving}
            onClick={() => void load()}
          >
            {t("common.reset")}
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}
