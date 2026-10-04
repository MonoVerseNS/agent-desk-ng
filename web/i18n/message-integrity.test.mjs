import assert from "node:assert/strict"
import test from "node:test"
import { readFile, readdir } from "node:fs/promises"
import path from "node:path"

const WEB_ROOT = new URL("../", import.meta.url)
const LOCALES = ["zh-CN", "en-US", "ru-RU"]
const SOURCE_DIRS = ["app", "components", "lib"]
const SKIP_DIRS = new Set(["node_modules", ".next", "out", "public"])

async function collectSourceFiles(dir) {
  const entries = await readdir(new URL(dir, WEB_ROOT), { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    if (entry.isDirectory()) {
      if (!SKIP_DIRS.has(entry.name)) files.push(...(await collectSourceFiles(path.posix.join(dir, entry.name))))
      continue
    }
    if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.mjs$/.test(entry.name)) {
      files.push(path.posix.join(dir, entry.name))
    }
  }
  return files
}

/**
 * Finds duplicate keys inside any JSON object. JSON.parse keeps only the last
 * occurrence, so a repeated top-level key silently deletes every sibling of the
 * block that shadowed it - that is how the workflowRun namespace lost 39 keys.
 */
function findDuplicateKeys(text) {
  const stack = []
  const trail = []
  const duplicates = []
  let index = 0
  let inString = false
  let escaped = false
  let stringStart = -1
  let expectingKey = false

  while (index < text.length) {
    const char = text[index]

    if (inString) {
      if (escaped) escaped = false
      else if (char === "\\") escaped = true
      else if (char === '"') {
        inString = false
        const raw = text.slice(stringStart + 1, index)
        const top = stack[stack.length - 1]
        if (expectingKey && top && !top.isArray) {
          if (top.keys.has(raw)) duplicates.push([...trail, raw].join("."))
          else top.keys.add(raw)
          expectingKey = false
        }
      }
      index += 1
      continue
    }

    if (char === '"') {
      inString = true
      escaped = false
      stringStart = index
      index += 1
      continue
    }
    if (char === "{") {
      stack.push({ isArray: false, keys: new Set() })
      trail.push("<object>")
      expectingKey = true
    } else if (char === "[") {
      stack.push({ isArray: true, keys: new Set() })
      trail.push("<array>")
      expectingKey = false
    } else if (char === "}" || char === "]") {
      stack.pop()
      trail.pop()
      expectingKey = false
    } else if (char === ",") {
      const top = stack[stack.length - 1]
      expectingKey = Boolean(top) && !top.isArray
    } else if (char === ":") {
      expectingKey = false
    }
    index += 1
  }

  return duplicates
}

test("no message bundle declares the same key twice", async () => {
  for (const locale of LOCALES) {
    const source = await readFile(new URL(`messages/${locale}.json`, WEB_ROOT), "utf8")
    assert.deepEqual(findDuplicateKeys(source), [], `${locale}.json has duplicate keys`)
  }
})

// Keys the UI resolves through t()/translateMessage(), plus the data-driven
// titleKey-style props. Scanning only t() calls misses lib/navigation.tsx,
// which is where the reported sidebar gaps lived.
const KEY_PATTERNS = [
  /\b(?:t|translateCurrentMessage|translateMessage)\(\s*["'`]([A-Za-z][\w]*(?:\.[A-Za-z][\w]*)+)["'`]/g,
  /\b(?:titleKey|labelKey|descriptionKey|textKey|placeholderKey|ariaLabelKey|tooltipKey):\s*["'`]([A-Za-z][\w]*(?:\.[A-Za-z][\w]*)+)["'`]/g,
]

test("every key the UI asks for exists in every locale", async () => {
  const bundles = {}
  for (const locale of LOCALES) {
    bundles[locale] = JSON.parse(await readFile(new URL(`messages/${locale}.json`, WEB_ROOT), "utf8"))
  }
  const lookup = (bundle, key) => key.split(".").reduce((value, segment) => (value == null ? undefined : value[segment]), bundle)

  const files = (await Promise.all(SOURCE_DIRS.map((dir) => collectSourceFiles(dir)))).flat()
  const missing = []

  for (const file of files) {
    const source = await readFile(new URL(file, WEB_ROOT), "utf8")
    for (const pattern of KEY_PATTERNS) {
      for (const match of source.matchAll(pattern)) {
        for (const locale of LOCALES) {
          if (lookup(bundles[locale], match[1]) === undefined) {
            missing.push(`${file}: ${match[1]} (missing in ${locale})`)
          }
        }
      }
    }
  }

  assert.deepEqual([...new Set(missing)], [])
})

// Template keys such as `supportPublic.comment.sort.${sort}` cannot be resolved
// by scanning source, so their known value sets are pinned here.
const DYNAMIC_KEYS = {
  "supportPublic.comment.sort": ["default", "hot", "latest"],
  "supportPublic.help": ["previous", "next"],
  supportChat: ["connecting", "connected", "disconnected"],
  agentTeamSchedule: ["weekdayMon", "weekdayTue", "weekdayWed", "weekdayThu", "weekdayFri", "weekdaySat", "weekdaySun"],
  supportHelpWorkbench: [
    "publishedAction",
    "publishedConfirmTitle",
    "publishedConfirmDescription",
    "publishedSuccess",
    "draftAction",
    "draftConfirmTitle",
    "draftConfirmDescription",
    "draftSuccess",
    "hiddenAction",
    "hiddenConfirmTitle",
    "hiddenConfirmDescription",
    "hiddenSuccess",
  ],
}

test("every dynamically composed key variant exists in every locale", async () => {
  const bundles = {}
  for (const locale of LOCALES) {
    bundles[locale] = JSON.parse(await readFile(new URL(`messages/${locale}.json`, WEB_ROOT), "utf8"))
  }
  const lookup = (bundle, key) => key.split(".").reduce((value, segment) => (value == null ? undefined : value[segment]), bundle)

  const missing = []
  for (const [prefix, variants] of Object.entries(DYNAMIC_KEYS)) {
    for (const variant of variants) {
      for (const locale of LOCALES) {
        if (lookup(bundles[locale], `${prefix}.${variant}`) === undefined) {
          missing.push(`${prefix}.${variant} (missing in ${locale})`)
        }
      }
    }
  }
  assert.deepEqual(missing, [])
})