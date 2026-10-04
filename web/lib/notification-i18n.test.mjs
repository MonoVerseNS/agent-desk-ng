import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

const messageBundles = {}

async function loadBundle(locale) {
  if (!messageBundles[locale]) {
    const source = await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")
    messageBundles[locale] = JSON.parse(source)
  }
  return messageBundles[locale]
}

function lookupMessage(bundle, key) {
  return key.split(".").reduce((value, segment) => (value == null ? undefined : value[segment]), bundle)
}

function translateMessage(locale, key, values) {
  const message = lookupMessage(messageBundles[locale], key)
  if (typeof message !== "string") {
    return key
  }
  if (!values) {
    return message
  }
  return message.replace(/\{(\w+)\}/g, (match, name) => String(values[name] ?? match))
}

async function loadModule() {
  await loadBundle("en-US")
  await loadBundle("ru-RU")
  const source = await readFile(new URL("./notification-i18n.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "notification-i18n.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (id) => {
      if (id === "@/i18n/config") {
        return { DEFAULT_LOCALE: "zh-CN", normalizeLocale: normalizeLocaleStub }
      }
      if (id === "@/i18n/messages") {
        return { translateMessage }
      }
      throw new Error(`unexpected import ${id}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

function normalizeLocaleStub(value) {
  const key = String(value ?? "").trim().toLowerCase()
  if (key === "zh-cn" || key === "zh_cn" || key === "zh") {
    return "zh-CN"
  }
  if (key === "en-us" || key === "en_us" || key === "en") {
    return "en-US"
  }
  if (key === "ru-ru" || key === "ru_ru" || key === "ru") {
    return "ru-RU"
  }
  return "zh-CN"
}

const ticketAssignedNotification = {
  id: 1,
  recipientUserId: 2,
  title: "工单指派提醒",
  content: "工单 TK-100 已指派给你\nCannot sign in\n指派原因: urgent",
  notificationType: "ticket_assigned",
  bizType: "ticket",
  bizId: 100,
  actionUrl: "/dashboard/tickets?ticketId=100",
}

test("localizes realtime ticket assignment notification to English", async () => {
  const { localizeNotificationItem } = await loadModule()

  const result = localizeNotificationItem(ticketAssignedNotification, "en-US")

  assert.equal(result.title, "Ticket assigned")
  assert.equal(
    result.content,
    "Ticket TK-100 has been assigned to you.\nCannot sign in\nAssignment reason: urgent"
  )
})

test("localizes realtime ticket assignment notification to Russian", async () => {
  await loadBundle("ru-RU")
  const { localizeNotificationItem } = await loadModule()

  const result = localizeNotificationItem(ticketAssignedNotification, "ru-RU")

  assert.equal(result.title, "Обращение назначено")
  assert.equal(
    result.content,
    "Обращение TK-100 назначено на вас.\nCannot sign in\nПричина назначения: urgent"
  )
})

test("leaves Chinese notifications untouched for the Chinese locale", async () => {
  const { localizeNotificationItem } = await loadModule()

  const result = localizeNotificationItem(ticketAssignedNotification, "zh-CN")

  assert.equal(result.title, ticketAssignedNotification.title)
  assert.equal(result.content, ticketAssignedNotification.content)
})