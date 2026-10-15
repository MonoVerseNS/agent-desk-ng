"use client"

import { useCallback, useEffect, useState } from "react"
import { MessageSquareTextIcon } from "lucide-react"
import { toast } from "sonner"

import { DashboardCrudPage } from "@/components/dashboard/crud"
import { DashboardPage } from "@/components/dashboard-page"
import { OptionCombobox } from "@/components/option-combobox"
import { ProjectDialog } from "@/components/project-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Textarea } from "@/components/ui/textarea"
import {
  acceptCommunityCommentAdmin,
  createCommunityCommentAdmin,
  deleteCommunityCategoryAdmin,
  fetchCommunityPostAdmin,
  fetchCommunityCategoriesAllAdmin,
  fetchCommunityCategoriesAdmin,
  fetchCommunityPostsAdmin,
  moderateCommunityPostAdmin,
  saveCommunityCategoryAdmin,
  updateCommunityCategorySortAdmin,
  type AdminCategory,
  type AdminPost,
  type AdminPostDetail,
} from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"
import { formatDateTime } from "@/lib/utils"
import { normalizeSupportSlug, supportSlugPattern } from "@/lib/support-slug"

function getPostStatusOptions(t: (key: string) => string) {
  return [
    { value: "all", label: t("supportCommunityAdmin.allStatuses") },
    { value: "pending", label: t("supportCommunityAdmin.statusPending") },
    { value: "normal", label: t("supportCommunityAdmin.statusNormal") },
    { value: "resolved", label: t("supportCommunityAdmin.statusResolved") },
    { value: "closed", label: t("supportCommunityAdmin.statusClosed") },
    { value: "hidden", label: t("supportCommunityAdmin.statusHidden") },
  ]
}

type CategoryPayload = Pick<
  AdminCategory,
  "name" | "slug" | "description" | "status" | "remark"
>

function postStatusLabel(status: string, t: (key: string) => string) {
  return getPostStatusOptions(t).find((item) => item.value === status)?.label ?? status
}

export function DashboardSupportCommunityCategoryAdmin() {
  const t = useI18n()
  const categoryStatusOptions = [
    { value: "all", label: t("supportCommunityCategory.allStatuses") },
    { value: "0", label: t("supportCommunityCategory.enabled") },
    { value: "1", label: t("supportCommunityCategory.disabled") },
  ]
  const crudFormLabels = {
    create: t("supportCommunityCategory.create"),
    save: t("supportCommunityCategory.save"),
    saving: t("supportCommunityCategory.saving"),
    cancel: t("supportCommunityCategory.cancel"),
    loadingDetail: t("supportCommunityCategory.loadingDetail"),
    required: t("supportCommunityCategory.required"),
    invalidNumber: t("supportCommunityCategory.invalidNumber"),
    minValue: (min: number) => t("supportCommunityCategory.minValue", { min }),
    maxValue: (max: number) => t("supportCommunityCategory.maxValue", { max }),
  }

  return (
    <DashboardCrudPage<AdminCategory, CategoryPayload>
      filters={[
        { name: "name", label: t("supportCommunityCategory.name"), placeholder: t("supportCommunityCategory.searchName"), defaultValue: "", trim: true, className: "w-full sm:w-72" },
        { name: "status", label: t("supportCommunityCategory.allStatuses"), placeholder: t("supportCommunityCategory.allStatuses"), type: "select", defaultValue: "all", allValue: "all", valueType: "number", options: categoryStatusOptions, className: "w-full sm:w-36" },
      ]}
      columns={[
        { key: "name", label: t("supportCommunityCategory.name"), render: (item) => <div><div className="font-medium">{item.name}</div><div className="mt-1 text-sm text-muted-foreground">{item.description || t("supportCommunityCategory.noDescription")}</div></div> },
        { key: "slug", label: "Slug", className: "w-48", render: (item) => <span className="font-mono text-sm">{item.slug}</span> },
        { key: "status", label: t("supportCommunityCategory.status"), className: "w-28", render: (item) => <Badge variant={item.status === 0 ? "default" : "outline"}>{item.status === 0 ? t("supportCommunityCategory.enabled") : t("supportCommunityCategory.disabled")}</Badge> },
      ]}
      fetchList={fetchCommunityCategoriesAdmin}
      getItemId={(item) => item.id}
      createItem={(payload) => saveCommunityCategoryAdmin(payload)}
      updateItem={(item, payload) => saveCommunityCategoryAdmin({ id: item.id, ...payload })}
      deleteItem={(item) => deleteCommunityCategoryAdmin(item.id)}
      deleteConfirm={(item) => ({ title: t("supportCommunityCategory.confirmDeleteTitle"), description: t("supportCommunityCategory.confirmDeleteDescription", { name: item.name }), confirmText: t("supportCommunityCategory.delete"), variant: "destructive" })}
      sort={{
        enabled: true,
        onReorder: (items) => updateCommunityCategorySortAdmin(items.map((item) => item.id)),
        successMessage: t("supportCommunityCategory.sortUpdated"),
        errorMessage: t("supportCommunityCategory.sortUpdateFailed"),
        handleLabel: t("supportCommunityCategory.dragSort", { name: "" }),
      }}
      form={{
        fields: [
          { name: "name", label: t("supportCommunityCategory.name"), placeholder: t("supportCommunityCategory.namePlaceholder"), required: true, trim: true },
          { name: "slug", label: "Slug", placeholder: t("supportCommunityCategory.slugPlaceholder"), required: true, trim: true, normalizeInput: normalizeSupportSlug, pattern: supportSlugPattern, patternMessage: t("supportCommunityCategory.slugPatternMessage") },
          { name: "description", label: t("supportCommunityCategory.description"), placeholder: t("supportCommunityCategory.descriptionPlaceholder"), type: "textarea", rows: 3, trim: true },
          { name: "status", label: t("supportCommunityCategory.status"), type: "select", defaultValue: "0", valueType: "number", required: true, options: categoryStatusOptions.filter((item) => item.value !== "all"), valueFromItem: (item) => String(item.status) },
          { name: "remark", label: t("supportCommunityCategory.remark"), placeholder: t("supportCommunityCategory.remarkPlaceholder"), type: "textarea", rows: 3, trim: true },
        ],
        transformSubmitValues: (values) => ({ name: String(values.name), slug: normalizeSupportSlug(String(values.slug)), description: String(values.description ?? ""), status: Number(values.status), remark: String(values.remark ?? "") }),
        labels: { ...crudFormLabels, createTitle: t("supportCommunityCategory.createTitle"), editTitle: t("supportCommunityCategory.editTitle") },
      }}
      labels={{
        refresh: t("supportCommunityCategory.refresh"), create: t("supportCommunityCategory.new"), query: t("supportCommunityCategory.query"), loading: t("supportCommunityCategory.loading"), empty: t("supportCommunityCategory.empty"), actions: t("supportCommunityCategory.actions"), edit: t("supportCommunityCategory.edit"), delete: t("supportCommunityCategory.delete"), processing: t("supportCommunityCategory.processing"), moreActions: (item) => t("supportCommunityCategory.moreActions", { name: item.name }), loadFailed: t("supportCommunityCategory.loadFailed"), saveFailed: t("supportCommunityCategory.saveFailed"), deleteFailed: t("supportCommunityCategory.deleteFailed"), created: (payload) => t("supportCommunityCategory.created", { name: payload.name }), updated: (item) => t("supportCommunityCategory.updated", { name: item.name }), deleted: (item) => t("supportCommunityCategory.deleted", { name: item.name }),
      }}
    />
  )
}

export function DashboardSupportCommunityAdmin() {
  const t = useI18n()
  const [categories, setCategories] = useState<AdminCategory[]>([])
  const [posts, setPosts] = useState<AdminPost[]>([])
  const [categoryId, setCategoryId] = useState<number | "all">("all")
  const [status, setStatus] = useState("all")
  const [detail, setDetail] = useState<AdminPostDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [comment, setComment] = useState("")

  const reloadPosts = useCallback(async () => {
    setLoading(true)
    try {
      const page = await fetchCommunityPostsAdmin({ categoryId: categoryId === "all" ? undefined : categoryId, status: status === "all" ? undefined : status, limit: 50 })
      setPosts(page.results)
    } finally {
      setLoading(false)
    }
  }, [categoryId, status])

  useEffect(() => { void fetchCommunityCategoriesAllAdmin().then(setCategories) }, [])
  useEffect(() => { void reloadPosts() }, [reloadPosts])

  const openPost = async (id: number) => setDetail(await fetchCommunityPostAdmin(id))
  const refreshDetail = async () => { if (detail) setDetail(await fetchCommunityPostAdmin(detail.post.id)) }

  const moderate = async (nextStatus: string) => {
    if (!detail) return
    await moderateCommunityPostAdmin(detail.post.id, nextStatus)
    toast.success(t("supportCommunityAdmin.postStatusUpdated"))
    await Promise.all([refreshDetail(), reloadPosts()])
  }

  const postStatusOptions = getPostStatusOptions(t)

  const reply = async () => {
    if (!detail || !comment.trim()) return
    await createCommunityCommentAdmin(detail.post.id, comment.trim())
    setComment("")
    toast.success(t("supportCommunityAdmin.commentSuccess"))
    await Promise.all([refreshDetail(), reloadPosts()])
  }

  return (
    <DashboardPage>
      <div className="flex flex-wrap items-center gap-2 border-b pb-3">
        <div className="w-full sm:w-44">
          <OptionCombobox
            value={String(categoryId)}
            onChange={(value) => setCategoryId(value === "all" ? "all" : Number(value))}
            placeholder={t("supportCommunityAdmin.allCategories")}
            options={[{ value: "all", label: t("supportCommunityAdmin.allCategories") }, ...categories.map((item) => ({ value: String(item.id), label: item.name }))]}
          />
        </div>
        <div className="w-full sm:w-36">
          <OptionCombobox
            value={status}
            onChange={setStatus}
            placeholder={t("supportCommunityAdmin.allStatuses")}
            options={postStatusOptions}
          />
        </div>
        <Button variant="outline" onClick={() => void reloadPosts()} disabled={loading}>{t("supportCommunityAdmin.refresh")}</Button>
      </div>

      <div className="overflow-hidden rounded-md border">
        {posts.map((post) => (
          <button key={post.id} type="button" className="flex w-full items-center gap-4 border-b px-4 py-3 text-left last:border-b-0 hover:bg-muted/50" onClick={() => void openPost(post.id)}>
            <span className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted"><MessageSquareTextIcon className="size-4 text-muted-foreground" /></span>
            <span className="min-w-0 flex-1"><span className="block truncate font-medium">{post.title}</span><span className="mt-1 block text-sm text-muted-foreground">{post.userName || t("supportCommunityAdmin.defaultUser")} · {post.categoryName || t("supportCommunityAdmin.uncategorized")} · {formatDateTime(post.createdAt)}</span></span>
            <span className="hidden text-sm text-muted-foreground sm:block">{t("supportCommunityAdmin.commentCount", { count: post.commentCount })}</span>
            <Badge variant={post.status === "resolved" ? "default" : "outline"}>{postStatusLabel(post.status, t)}</Badge>
          </button>
        ))}
        {!loading && posts.length === 0 ? <div className="py-16 text-center text-sm text-muted-foreground">{t("supportCommunityAdmin.noMatchingPosts")}</div> : null}
        {loading ? <div className="py-16 text-center text-sm text-muted-foreground">{t("supportCommunityAdmin.loadingPosts")}</div> : null}
      </div>

      <ProjectDialog open={Boolean(detail)} onOpenChange={(open) => { if (!open) setDetail(null) }} title={detail?.post.title ?? t("supportCommunityAdmin.moderateDialogTitle")} description={detail ? `${detail.post.userName || t("supportCommunityAdmin.defaultUser")} · ${detail.post.categoryName || t("supportCommunityAdmin.uncategorized")} · ${formatDateTime(detail.post.createdAt)}` : undefined} size="xl" allowFullscreen>
        {detail ? (
          <div className="grid gap-5">
            <div className="flex flex-wrap items-center gap-2"><Badge variant={detail.post.status === "resolved" ? "default" : "outline"}>{postStatusLabel(detail.post.status, t)}</Badge><span className="text-sm text-muted-foreground">{t("supportCommunityAdmin.likesAndViews", { likes: detail.post.reactionCount, views: detail.post.viewCount })}</span></div>
            <p className="whitespace-pre-wrap text-sm leading-7">{detail.post.content}</p>
            <div className="flex flex-wrap gap-2 border-y py-3">
              <Button variant="outline" onClick={() => void moderate("normal")}>{t("supportCommunityAdmin.restoreNormalAction")}</Button>
              <Button variant="outline" onClick={() => void moderate("closed")}>{t("supportCommunityAdmin.closePostAction")}</Button>
              <Button variant="destructive" onClick={() => void moderate("hidden")}>{t("supportCommunityAdmin.hidePostAction")}</Button>
            </div>
            <Card><CardHeader><CardTitle className="text-base">{t("supportCommunityAdmin.commentsTitle", { count: detail.comments.length })}</CardTitle></CardHeader><CardContent className="grid gap-3">{detail.comments.map((item) => <div key={item.id} className="rounded-md border p-3"><div className="flex items-center justify-between gap-3"><span className="text-sm font-medium">{item.authorName || item.authorType}</span>{item.isAccepted ? <Badge>{t("supportCommunityAdmin.acceptedBadge")}</Badge> : <Button size="sm" variant="outline" onClick={() => void acceptCommunityCommentAdmin(detail.post.id, item.id).then(refreshDetail)}>{t("supportCommunityAdmin.acceptAnswerAction")}</Button>}</div><p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{item.content}</p></div>)}</CardContent></Card>
            <div className="grid gap-2"><Textarea value={comment} onChange={(event) => setComment(event.target.value)} rows={5} placeholder={t("supportCommunityAdmin.commentPlaceholder")} /><div className="flex justify-end"><Button onClick={() => void reply()} disabled={!comment.trim()}>{t("supportCommunityAdmin.publishCommentAction")}</Button></div></div>
          </div>
        ) : null}
      </ProjectDialog>
    </DashboardPage>
  )
}
