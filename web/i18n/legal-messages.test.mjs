import assert from "node:assert/strict"
import test from "node:test"
import { readFile } from "node:fs/promises"

const SUPPORTED_LOCALES = ["zh-CN", "en-US", "ru-RU"]

async function loadMessages(locale) {
  const source = await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")
  return JSON.parse(source)
}

function flatten(source, prefix = "", target = {}) {
  for (const [key, value] of Object.entries(source)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === "object" && !Array.isArray(value)) {
      flatten(value, path, target)
      continue
    }
    target[path] = value
  }
  return target
}

function placeholders(value) {
  return ((String(value).match(/\{(\w+)\}/g) ?? []).slice().sort().join("|"))
}

test("legal pages have localized document content", async () => {
  for (const locale of SUPPORTED_LOCALES) {
    const messages = await loadMessages(locale)

    assert.equal(typeof messages.legal.terms.title, "string")
    assert.equal(typeof messages.legal.privacy.title, "string")
    assert.ok(messages.legal.terms.sections.length >= 6)
    assert.ok(messages.legal.privacy.sections.length >= 6)

    for (const page of [messages.legal.terms, messages.legal.privacy]) {
      assert.equal(typeof page.updatedAt, "string")
      assert.equal(typeof page.relatedLabel, "string")
      assert.equal(typeof page.relatedLink, "string")
      for (const section of page.sections) {
        assert.equal(typeof section.title, "string")
        assert.equal(typeof section.body, "string")
      }
    }
  }
})

test("every locale bundle defines the same keys as the default locale", async () => {
  const base = flatten(await loadMessages("zh-CN"))

  for (const locale of SUPPORTED_LOCALES) {
    const messages = flatten(await loadMessages(locale))

    for (const key of Object.keys(base)) {
      assert.ok(key in messages, `${locale}.json is missing key ${key}`)
    }
    for (const key of Object.keys(messages)) {
      assert.ok(key in base, `${locale}.json has extra key ${key}`)
    }
  }
})

test("every locale keeps the same interpolation placeholders", async () => {
  const base = flatten(await loadMessages("zh-CN"))

  for (const locale of SUPPORTED_LOCALES) {
    const messages = flatten(await loadMessages(locale))

    for (const [key, value] of Object.entries(base)) {
      if (typeof value !== "string" || Array.isArray(value)) {
        continue
      }
      assert.equal(
        placeholders(messages[key]),
        placeholders(value),
        `${locale}.json key ${key} placeholders differ`
      )
    }
  }
})

test("locale labels exist for every supported locale", async () => {
  for (const locale of SUPPORTED_LOCALES) {
    const messages = await loadMessages(locale)

    for (const supported of SUPPORTED_LOCALES) {
      assert.equal(typeof messages.locale[supported], "string")
    }
  }
})