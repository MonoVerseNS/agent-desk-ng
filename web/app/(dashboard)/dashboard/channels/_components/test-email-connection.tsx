"use client"

import { useState } from "react"
import { PlugZapIcon } from "lucide-react"
import { toast } from "sonner"

import { Button } from "@/components/ui/button"
import { testChannelEmail } from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"

type TestEmailConnectionButtonProps = {
  /** Undefined for a channel that has not been saved yet. */
  channelId?: number | null
}

/**
 * Verifies that the channel's bound mail server is reachable before anyone relies
 * on it. Saving a channel cannot tell a wrong host from a right one, and the
 * alternative is finding out by filing a real request and watching the outbox
 * retry five times.
 */
export function TestEmailConnectionButton({
  channelId,
}: TestEmailConnectionButtonProps) {
  const t = useI18n()
  const [testing, setTesting] = useState(false)

  async function handleTest() {
    if (testing) {
      return
    }
    if (!channelId) {
      toast.error(t("channel.smtpTestSaveFirst"))
      return
    }

    setTesting(true)
    try {
      const result = await testChannelEmail(channelId)
      toast.success(
        t("channel.smtpTestSuccess", {
          host: `${result.host}:${result.port}`,
        })
      )
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t("channel.smtpTestFailed")
      )
    } finally {
      setTesting(false)
    }
  }

  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={testing}
      onClick={() => void handleTest()}
      className="w-full"
    >
      <PlugZapIcon className="size-4" />
      {testing ? t("channel.smtpTesting") : t("channel.smtpTest")}
    </Button>
  )
}