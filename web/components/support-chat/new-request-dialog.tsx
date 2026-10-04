"use client"

import { useState } from "react"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useSupportChatStore } from "@/lib/stores/support-chat"
import { useI18n } from "@/i18n/provider"

const SUBJECT_MAX_LENGTH = 255

type SupportChatNewRequestDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated?: () => void
}

/**
 * Asks for an optional subject before starting a request. The subject is what
 * tells several requests apart in the switcher, so it is worth prompting for -
 * but a visitor in a hurry must not be blocked, hence "optional" and an
 * immediate path through.
 */
export function SupportChatNewRequestDialog({
  open,
  onOpenChange,
  onCreated,
}: SupportChatNewRequestDialogProps) {
  const t = useI18n()
  const creatingConversation = useSupportChatStore(
    (state) => state.creatingConversation
  )
  const startNewConversation = useSupportChatStore(
    (state) => state.startNewConversation
  )
  const [subject, setSubject] = useState("")
  const [validationError, setValidationError] = useState("")
  const [submitFailed, setSubmitFailed] = useState(false)

  function resetForm() {
    setSubject("")
    setValidationError("")
    setSubmitFailed(false)
  }

  // The dialog body unmounts whenever it is closed, so resetting on close rather
  // than in an effect keyed on `open` also avoids a cascading render.
  function handleOpenChange(nextOpen: boolean) {
    if (creatingConversation) {
      return
    }
    if (!nextOpen) {
      resetForm()
    }
    onOpenChange(nextOpen)
  }

  async function handleCreate() {
    const trimmed = subject.trim()
    if ([...trimmed].length > SUBJECT_MAX_LENGTH) {
      setValidationError(t("supportChat.newRequestSubjectTooLong"))
      return
    }

    setValidationError("")
    setSubmitFailed(false)
    try {
      await startNewConversation(trimmed)
      onCreated?.()
      resetForm()
      onOpenChange(false)
    } catch {
      // The store already surfaced the localized reason on its error strip;
      // keeping the dialog open preserves what the visitor typed.
      setSubmitFailed(true)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="max-w-[380px]"
        showCloseButton={!creatingConversation}
      >
        <DialogHeader>
          <DialogTitle>{t("supportChat.newRequestDialogTitle")}</DialogTitle>
          <DialogDescription>
            {t("supportChat.newRequestDialogDescription")}
          </DialogDescription>
        </DialogHeader>

        <Field className="gap-2">
          <FieldLabel htmlFor="support-chat-new-request-subject">
            {t("supportChat.newRequestSubjectLabel")}
          </FieldLabel>
          <Input
            id="support-chat-new-request-subject"
            value={subject}
            maxLength={SUBJECT_MAX_LENGTH}
            placeholder={t("supportChat.newRequestSubjectPlaceholder")}
            disabled={creatingConversation}
            aria-invalid={validationError ? true : undefined}
            aria-describedby={
              validationError
                ? "support-chat-new-request-subject-error"
                : "support-chat-new-request-subject-hint"
            }
            onChange={(event) => {
              setSubject(event.target.value)
              if (validationError) {
                setValidationError("")
              }
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !creatingConversation) {
                event.preventDefault()
                void handleCreate()
              }
            }}
          />
          {validationError ? (
            <p
              id="support-chat-new-request-subject-error"
              className="text-xs text-destructive"
            >
              {validationError}
            </p>
          ) : (
            <FieldDescription id="support-chat-new-request-subject-hint">
              {t("supportChat.newRequestSubjectHint")}
            </FieldDescription>
          )}
          {submitFailed ? (
            <p className="text-xs text-destructive">
              {t("supportChat.createConversationFailed")}
            </p>
          ) : null}
        </Field>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            disabled={creatingConversation}
            onClick={() => handleOpenChange(false)}
          >
            {t("common.cancel")}
          </Button>
          <Button
            type="button"
            disabled={creatingConversation}
            onClick={() => void handleCreate()}
          >
            {creatingConversation
              ? t("supportChat.creatingRequest")
              : t("supportChat.createRequest")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}