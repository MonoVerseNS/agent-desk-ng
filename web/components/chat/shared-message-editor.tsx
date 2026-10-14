"use client"

import { useEffect, useRef, useState, type ChangeEvent } from "react"
import Placeholder from "@tiptap/extension-placeholder"
import { EditorContent, useEditor } from "@tiptap/react"
import StarterKit from "@tiptap/starter-kit"
import {
  ImageIcon,
  MessageSquareTextIcon,
  PaperclipIcon,
  SendHorizonalIcon,
  SendIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
  buildSendableEditorHTML,
  hasUploadingEditorImages,
  markEditorImageUploadedByTitle,
  MessageImageExtension,
  removeEditorImageByTitle,
  revokeEditorObjectUrl,
  revokeEditorObjectUrls,
  setEditorImageUploadingByTitle,
  type UploadedEditorImage,
} from "@/lib/im-editor-image"
import { generateUUID } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

export type UploadedMessageEditorImage = UploadedEditorImage & {
  url: string
}

export type MessageEditorQuickReply = {
  id: number
  groupName?: string
  title: string
  content: string
}

type SharedMessageEditorVariant = "customer" | "agent"

type SharedMessageEditorProps = {
  variant: SharedMessageEditorVariant
  disabled?: boolean
  uploadingAsset?: boolean
  manageLocalUploading?: boolean
  quickReplies?: {
    open: boolean
    loading: boolean
    items: MessageEditorQuickReply[]
    onOpenChange: (open: boolean) => void
  }
  onSend: (html: string) => Promise<void>
  onUploadImage: (file: File) => Promise<UploadedMessageEditorImage | null>
  onSendAttachment: (file: File) => Promise<void>
}

export function SharedMessageEditor({
  variant,
  disabled = false,
  uploadingAsset = false,
  manageLocalUploading = false,
  quickReplies,
  onSend,
  onUploadImage,
  onSendAttachment,
}: SharedMessageEditorProps) {
  const t = useI18n()
  const [localUploading, setLocalUploading] = useState(false)
  const [quickReplySearch, setQuickReplySearch] = useState("")
  const quickReplySearchRef = useRef(quickReplySearch)
  quickReplySearchRef.current = quickReplySearch
  const isSlashTriggeredRef = useRef(false)
  const quickRepliesRef = useRef(quickReplies)
  const imageInputRef = useRef<HTMLInputElement | null>(null)
  const attachmentInputRef = useRef<HTMLInputElement | null>(null)
  const onSendRef = useRef(onSend)
  const onUploadImageRef = useRef(onUploadImage)
  const onSendAttachmentRef = useRef(onSendAttachment)
  const shouldRestoreFocusRef = useRef(false)
  const objectUrlsRef = useRef<Set<string>>(new Set())
  const uploadedImagesRef = useRef(new Map<string, UploadedMessageEditorImage>())
  const placeholderRef = useRef(t("conversation.editorPlaceholder"))
  const isCustomer = variant === "customer"
  const isUploading = uploadingAsset || (manageLocalUploading && localUploading)

  placeholderRef.current = t("conversation.editorPlaceholder")

  useEffect(() => {
    quickRepliesRef.current = quickReplies
  }, [quickReplies])

  useEffect(() => {
    const objectUrls = objectUrlsRef.current
    return () => {
      revokeEditorObjectUrls(objectUrls)
    }
  }, [])

  useEffect(() => {
    onSendRef.current = onSend
  }, [onSend])

  useEffect(() => {
    onUploadImageRef.current = onUploadImage
  }, [onUploadImage])

  useEffect(() => {
    onSendAttachmentRef.current = onSendAttachment
  }, [onSendAttachment])

  const editor = useEditor({
    immediatelyRender: false,
    extensions: [
      StarterKit.configure({
        heading: false,
        blockquote: false,
        codeBlock: false,
        bulletList: false,
        orderedList: false,
        horizontalRule: false,
      }),
      MessageImageExtension,
      Placeholder.configure({
        placeholder: () => placeholderRef.current,
      }),
    ],
    content: "",
    onUpdate: ({ editor }) => {
      if (!quickRepliesRef.current || quickRepliesRef.current.loading) {
        return
      }
      const { $from } = editor.state.selection
      const blockText = $from.parent.textContent
      const trimmed = blockText.trimStart()
      const match = trimmed.match(/^\/([^\s]*)$/)
      if (match) {
        isSlashTriggeredRef.current = true
        const query = match[1]
        setQuickReplySearch(query)
        if (!quickRepliesRef.current.open) {
          quickRepliesRef.current.onOpenChange(true)
        }
      } else if (isSlashTriggeredRef.current && quickRepliesRef.current.open) {
        isSlashTriggeredRef.current = false
        setQuickReplySearch("")
        quickRepliesRef.current.onOpenChange(false)
      }
    },
    editorProps: {
      attributes: {
        class: getEditorClassName(variant),
      },
      handleKeyDown: (_view, event) => {
        if (event.key === "Escape" && quickRepliesRef.current?.open) {
          event.preventDefault()
          quickRepliesRef.current.onOpenChange(false)
          setQuickReplySearch("")
          isSlashTriggeredRef.current = false
          return true
        }
        if (
          (event.key === "Enter" || event.key === "Tab") &&
          !event.shiftKey &&
          quickRepliesRef.current?.open
        ) {
          const query = quickReplySearchRef.current.toLowerCase().trim()
          const items = quickRepliesRef.current.items
          const matching = items.filter((item) => {
            if (!query) return true
            const target = `${item.groupName ?? ""} ${item.title} ${item.content}`.toLowerCase()
            return target.includes(query)
          })
          if (matching.length > 0) {
            event.preventDefault()
            handleInsertQuickReply(matching[0])
            return true
          }
          if (event.key === "Enter") {
            event.preventDefault()
            return true
          }
        }
        if (event.key === "Enter" && !event.shiftKey) {
          event.preventDefault()
          void handleSend()
          return true
        }
        return false
      },
      handlePaste: (_view, event) => {
        if (disabled || isUploading) {
          return false
        }
        const imageFile = getClipboardImageFile(event.clipboardData)
        if (!imageFile) {
          return false
        }
        event.preventDefault()
        void insertUploadedImage(imageFile)
        return true
      },
    },
  })

  useEffect(() => {
    if (!editor) {
      return
    }
    editor.setEditable(!disabled && !isUploading)
  }, [disabled, editor, isUploading])

  useEffect(() => {
    if (!editor || disabled || isUploading || !shouldRestoreFocusRef.current) {
      return
    }
    requestAnimationFrame(() => {
      editor.commands.focus()
    })
  }, [disabled, editor, isUploading])

  async function handleSend() {
    if (!editor || disabled || isUploading) {
      return
    }
    const rawHTML = editor.getHTML()
    if (hasUploadingEditorImages(rawHTML, uploadedImagesRef.current)) {
      return
    }
    const html = buildSendableEditorHTML(rawHTML, uploadedImagesRef.current)
    if (!isMeaningfulHTML(html)) {
      return
    }
    await onSendRef.current(html)
    editor.commands.clearContent(true)
    revokeEditorObjectUrls(objectUrlsRef.current)
    uploadedImagesRef.current.clear()
    setQuickReplySearch("")
    isSlashTriggeredRef.current = false
    quickRepliesRef.current?.onOpenChange(false)
    if (!isCustomer) {
      requestAnimationFrame(() => {
        editor.commands.focus("end")
      })
    }
  }

  async function handleSelectImage(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (!file || !editor || disabled || isUploading) {
      restoreFocusIfNeeded()
      return
    }
    await insertUploadedImage(file)
  }

  async function insertUploadedImage(file: File) {
    if (!editor || disabled || isUploading) {
      return
    }

    shouldRestoreFocusRef.current = true
    const objectUrl = URL.createObjectURL(file)
    objectUrlsRef.current.add(objectUrl)
    const placeholderId = `uploading-${generateUUID()}`
    editor
      .chain()
      .focus()
      .setImage({
        src: objectUrl,
        alt: file.name || "uploading-image",
        title: placeholderId,
      })
      .run()
    setEditorImageUploadingByTitle(editor, placeholderId)

    try {
      setLocalUploading(true)
      const uploaded = await onUploadImageRef.current(file)
      if (!uploaded?.assetId || !uploaded.provider || !uploaded.storageKey) {
        removeEditorImageByTitle(editor, placeholderId)
        revokeEditorObjectUrl(objectUrlsRef.current, objectUrl)
        return
      }
      markEditorImageUploadedByTitle(
        editor,
        placeholderId,
        uploaded,
        uploadedImagesRef.current
      )
    } finally {
      setLocalUploading(false)
      requestAnimationFrame(() => {
        if (!disabled && shouldRestoreFocusRef.current) {
          editor.commands.focus()
        }
      })
    }
  }

  async function handleSelectAttachment(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (!file || disabled || isUploading) {
      restoreFocusIfNeeded()
      return
    }

    shouldRestoreFocusRef.current = editor?.isFocused ?? true
    try {
      setLocalUploading(true)
      await onSendAttachmentRef.current(file)
    } finally {
      setLocalUploading(false)
      requestAnimationFrame(() => {
        if (editor && !disabled && shouldRestoreFocusRef.current) {
          editor.commands.focus()
        }
      })
    }
  }

  function handleInsertQuickReply(item: MessageEditorQuickReply) {
    if (!editor || disabled || isUploading) {
      return
    }
    if (!item.content.trim()) {
      return
    }

    const { $from } = editor.state.selection
    const blockText = $from.parent.textContent
    const trimmed = blockText.trimStart()
    const match = trimmed.match(/^\/([^\s]*)$/)

    if (match) {
      const slashIndex = blockText.indexOf("/")
      const blockStart = $from.start() + slashIndex
      const matchLen = match[0].length
      editor
        .chain()
        .focus()
        .deleteRange({ from: blockStart, to: blockStart + matchLen })
        .insertContent(item.content)
        .run()
    } else {
      const fullText = editor.getText().trim()
      const fullMatch = fullText.match(/^\/([^\s]*)$/)
      if (fullMatch) {
        editor.chain().focus().setContent(item.content).run()
      } else {
        editor.chain().focus().insertContent(item.content).run()
      }
    }

    quickRepliesRef.current?.onOpenChange(false)
    setQuickReplySearch("")
    isSlashTriggeredRef.current = false
    requestAnimationFrame(() => {
      editor.commands.focus("end")
    })
  }

  function restoreFocusIfNeeded() {
    if (editor && shouldRestoreFocusRef.current) {
      requestAnimationFrame(() => {
        editor.commands.focus()
      })
    }
  }

  const editorContent = (
    <>
      <input
        ref={imageInputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={handleSelectImage}
      />
      <input
        ref={attachmentInputRef}
        type="file"
        className="hidden"
        onChange={handleSelectAttachment}
      />
      {isCustomer ? (
        <div className="min-h-10">
          <EditorContent editor={editor} />
        </div>
      ) : (
        <div className="min-h-0 flex-1 overflow-hidden px-2 py-1">
          <EditorContent editor={editor} className="h-full" />
        </div>
      )}
      <div className={getToolbarClassName(variant)}>
        <div className={isCustomer ? "flex items-center gap-1.5" : "flex items-center gap-1"}>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={getIconButtonClassName(variant)}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              shouldRestoreFocusRef.current = editor?.isFocused ?? true
              imageInputRef.current?.click()
            }}
            disabled={disabled || isUploading}
            aria-label={isUploading ? t("conversation.imageUploading") : t("conversation.sendImage")}
            title={isUploading ? t("conversation.imageUploading") : t("conversation.sendImage")}
          >
            <ImageIcon className={isCustomer ? undefined : "size-4"} />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={getIconButtonClassName(variant)}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              shouldRestoreFocusRef.current = editor?.isFocused ?? true
              attachmentInputRef.current?.click()
            }}
            disabled={disabled || isUploading}
            aria-label={isUploading ? t("conversation.attachmentUploading") : t("conversation.sendAttachment")}
            title={isUploading ? t("conversation.attachmentUploading") : t("conversation.sendAttachment")}
          >
            <PaperclipIcon className={isCustomer ? undefined : "size-4"} />
          </Button>
          {quickReplies ? (
            <Popover
              open={quickReplies.open}
              onOpenChange={(nextOpen) => {
                if (!nextOpen) {
                  setQuickReplySearch("")
                  isSlashTriggeredRef.current = false
                }
                quickReplies.onOpenChange(nextOpen)
              }}
            >
              <PopoverTrigger
                render={
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    disabled={disabled || isUploading || quickReplies.loading}
                    onMouseDown={(event) => event.preventDefault()}
                    title={t("conversation.quickReplySlashHint")}
                    aria-label={t("conversation.quickReplySlashHint")}
                  />
                }
              >
                <MessageSquareTextIcon className="size-4" />
              </PopoverTrigger>
              <PopoverContent
                className="w-[30rem] p-0"
                align="start"
                initialFocus={!isSlashTriggeredRef.current}
              >
                <Command>
                  <CommandInput
                    value={quickReplySearch}
                    onValueChange={setQuickReplySearch}
                    placeholder={t("conversation.searchQuickReplies")}
                  />
                  <CommandList>
                    <CommandEmpty>{t("conversation.emptyQuickReplies")}</CommandEmpty>
                    <CommandGroup>
                      {quickReplies.items.map((item) => (
                        <CommandItem
                          key={item.id}
                          value={`${item.groupName ?? ""} ${item.title} ${item.content}`}
                          onSelect={() => handleInsertQuickReply(item)}
                        >
                          <div className="flex min-w-0 flex-col gap-0.5 py-0.5">
                            <span className="line-clamp-1 text-sm">
                              {item.groupName
                                ? `${item.groupName} / ${item.title}`
                                : item.title}
                            </span>
                            <span className="line-clamp-2 text-xs text-muted-foreground">
                              {item.content}
                            </span>
                          </div>
                        </CommandItem>
                      ))}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          ) : null}
          {!isCustomer && quickReplies ? (
            <span className="hidden select-none text-[11px] text-muted-foreground/70 sm:inline-block pl-1">
              {t("conversation.quickReplySlashHint")}
            </span>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          <p className={isCustomer ? "hidden text-[10px] text-muted-foreground sm:block" : "text-xs text-muted-foreground"}>
            {t("conversation.enterToSend")}
          </p>
          {isCustomer ? (
            <Button
              type="button"
              size="icon"
              onClick={() => void handleSend()}
              disabled={disabled || isUploading}
              aria-label={t("conversation.send")}
              title={t("conversation.send")}
              className="bg-primary text-white shadow-[0_10px_20px_color-mix(in_srgb,var(--primary)_24%,transparent)] hover:bg-primary hover:brightness-105"
            >
              <SendHorizonalIcon />
            </Button>
          ) : (
            <Button
              type="button"
              size="sm"
              onClick={() => void handleSend()}
              disabled={disabled || isUploading}
            >
              <SendIcon className="mr-1 size-4" />
              {isUploading ? t("conversation.uploading") : t("conversation.send")}
            </Button>
          )}
        </div>
      </div>
    </>
  )

  if (isCustomer) {
    return (
      <div className="px-3 pt-2 pb-3">
        <div className="rounded-xl border border-border bg-background p-2 shadow-[0_8px_24px_rgba(15,23,42,0.05)] dark:shadow-none">
          {editorContent}
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col p-2">
      <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-sm border border-border bg-card">
        {editorContent}
      </div>
    </div>
  )
}

function getEditorClassName(variant: SharedMessageEditorVariant) {
  if (variant === "customer") {
    return "agent-desk-scrollbar min-h-12 max-h-40 overflow-y-auto px-1.5 py-1 text-sm leading-6 text-foreground outline-none [&_p]:m-0 [&_p+*]:mt-2 [&_.agent-desk-editor-image-wrap]:my-2 [&_.agent-desk-editor-image]:max-h-64 [&_.agent-desk-editor-image]:max-w-full [&_.agent-desk-editor-image]:rounded-lg [&_.agent-desk-editor-image]:object-contain [&_.agent-desk-editor-image-wrap-uploading_.agent-desk-editor-image]:opacity-55"
  }
  return "h-full min-h-12 max-h-[20vh] overflow-y-auto px-1.5 py-1 text-sm leading-6 text-foreground outline-none sm:max-h-none [&_.ProseMirror-focused]:outline-none [&_p]:m-0 [&_p+.agent-desk-editor-image-wrap]:mt-2 [&_.agent-desk-editor-image-wrap]:my-2 [&_.agent-desk-editor-image]:max-h-64 [&_.agent-desk-editor-image]:max-w-full [&_.agent-desk-editor-image]:rounded-md [&_.agent-desk-editor-image]:object-contain [&_.agent-desk-editor-image-wrap-uploading_.agent-desk-editor-image]:opacity-55 [&_p.is-editor-empty:first-child]:before:text-muted-foreground"
}

function getToolbarClassName(variant: SharedMessageEditorVariant) {
  if (variant === "customer") {
    return "mt-2 flex items-center justify-between"
  }
  return "flex items-center justify-between rounded-b-sm border-t border-border bg-card px-2 pt-1 pb-2"
}

function getIconButtonClassName(variant: SharedMessageEditorVariant) {
  if (variant === "customer") {
    return "text-muted-foreground hover:bg-muted hover:text-foreground"
  }
  return "size-8"
}

function isMeaningfulHTML(html: string) {
  const normalized = html
    .replace(/<p><\/p>/g, "")
    .replace(/<p><br><\/p>/g, "")
    .replace(/\s+/g, "")
  if (/<img[\s\S]*?>/i.test(normalized)) {
    return true
  }
  const plainText = normalized.replace(/<[^>]+>/g, "").trim()
  return plainText !== ""
}

function getClipboardImageFile(data: DataTransfer | null) {
  if (!data) {
    return null
  }

  for (const item of Array.from(data.items)) {
    if (item.kind === "file" && item.type.startsWith("image/")) {
      return item.getAsFile()
    }
  }
  return null
}
