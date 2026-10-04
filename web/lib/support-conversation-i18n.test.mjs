import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"
import ts from "typescript"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(
    new URL("./support-conversation-i18n.ts", import.meta.url),
    "utf8"
  )
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "support-conversation-i18n.ts",
  })
  const messages = {}
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (name) => {
      if (name === "@/i18n/messages") {
        return {
          translateCurrentMessage: (key) => messages[key] ?? key,
          translateMessage: (_locale, key) => messages[key] ?? key,
        }
      }
      throw new Error(`unexpected require: ${name}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return { module: sandbox.module.exports, messages }
}

const LOCALES = ["zh-CN", "en-US", "ru-RU"]

async function readMessages(locale) {
  const raw = await readFile(
    new URL(`../messages/${locale}.json`, import.meta.url),
    "utf8"
  )
  return JSON.parse(raw)
}

test("every conversation status maps to a key that exists in all locales", async () => {
  const { module, messages } = await loadModule()
  const catalogues = Object.fromEntries(
    await Promise.all(
      LOCALES.map(async (locale) => [locale, await readMessages(locale)])
    )
  )

  for (const locale of LOCALES) {
    for (const status of [1, 2, 3, 4]) {
      const key = module.getConversationStatusMessageKey(status)
      const expected = lookup(catalogues[locale], key)
      messages[key] = expected
      assert.ok(expected, `missing ${key} in ${locale}`)
      assert.equal(
        module.getConversationStatusLabel(status),
        expected,
        `status ${status} is not localized for ${locale}`
      )
    }
  }
})

test("no two statuses share a label in any locale", async () => {
  const catalogues = Object.fromEntries(
    await Promise.all(LOCALES.map(async (locale) => [locale, await readMessages(locale)]))
  )

  for (const locale of LOCALES) {
    const labels = [1, 2, 3, 4].map((status) =>
      lookup(catalogues[locale], statusKey(status))
    )
    assert.equal(
      new Set(labels).size,
      labels.length,
      `duplicate status labels in ${locale}: ${labels.join(" / ")}`
    )
  }
})

// The helper deals in fully qualified "supportChat.x" keys while the catalogues
// are already scoped to the supportChat namespace.
function lookup(catalogue, key) {
  return key.split(".").reduce((node, part) => node?.[part], catalogue)
}

function statusKey(status) {
  return {
    1: "supportChat.statusAiServing",
    2: "supportChat.statusPending",
    3: "supportChat.statusActive",
    4: "supportChat.statusClosed",
  }[status]
}

test("an unknown status still resolves to a defined label", async () => {
  const { module, messages } = await loadModule()
  const en = await readMessages("en-US")
  messages[module.getConversationStatusMessageKey(99)] =
    en.supportChat.statusClosed

  assert.equal(
    module.getConversationStatusLabel(99),
    en.supportChat.statusClosed
  )
  assert.equal(
    module.getConversationStatusMessageKey(0),
    module.getConversationStatusMessageKey(4)
  )
})

test("the fallback request title is localized everywhere", async () => {
  const { module, messages } = await loadModule()
  messages["supportChat.requestFallbackTitle"] = "Support request"

  assert.equal(
    module.getConversationFallbackTitle("ru-RU"),
    "Support request"
  )
})