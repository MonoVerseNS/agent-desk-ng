import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./permission-i18n.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "permission-i18n.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("localizes seeded permission display names to English", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("user.assignRole", "\u5206\u914d\u7528\u6237\u89d2\u8272", "en-US"), "Assign user roles")
  assert.equal(getPermissionDisplayName("agentTeamSchedule.batchGenerate", "\u6279\u91cf\u751f\u6210\u5ba2\u670d\u7ec4\u6392\u73ed", "en-US"), "Batch generate agent team schedules")
  assert.equal(getPermissionGroupName("agentTeamSchedule", "en-US"), "Agent team schedules")
})

test("keeps original permission names for Chinese locale", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("user.view", "\u67e5\u770b\u7528\u6237", "zh-CN"), "\u67e5\u770b\u7528\u6237")
  assert.equal(getPermissionGroupName("agentTeam", "zh-CN"), "agentTeam")
})

test("localizes seeded permission display names to Russian", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(
    getPermissionDisplayName("user.assignRole", "\u5206\u914d\u7528\u6237\u89d2\u8272", "ru-RU"),
    "\u041d\u0430\u0437\u043d\u0430\u0447\u0435\u043d\u0438\u0435 \u0440\u043e\u043b\u0435\u0439 \u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u0435\u043b\u044f\u043c"
  )
  assert.equal(getPermissionDisplayName("user.view", "\u67e5\u770b\u7528\u6237", "ru-RU"), "\u041f\u0440\u043e\u0441\u043c\u043e\u0442\u0440 \u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u0435\u043b\u0435\u0439")
  assert.equal(
    getPermissionDisplayName("agentTeamSchedule.batchGenerate", "\u6279\u91cf\u751f\u6210\u5ba2\u670d\u7ec4\u6392\u73ed", "ru-RU"),
    "\u041c\u0430\u0441\u0441\u043e\u0432\u043e\u0435 \u0441\u043e\u0437\u0434\u0430\u043d\u0438\u0435 \u0440\u0430\u0441\u043f\u0438\u0441\u0430\u043d\u0438\u0439 \u043a\u043e\u043c\u0430\u043d\u0434 \u043e\u043f\u0435\u0440\u0430\u0442\u043e\u0440\u043e\u0432"
  )
  assert.equal(getPermissionGroupName("agentTeamSchedule", "ru-RU"), "\u0420\u0430\u0441\u043f\u0438\u0441\u0430\u043d\u0438\u0439 \u043a\u043e\u043c\u0430\u043d\u0434 \u043e\u043f\u0435\u0440\u0430\u0442\u043e\u0440\u043e\u0432")
  assert.equal(getPermissionGroupName("aiAgent", "ru-RU"), "\u0418\u0418-\u0430\u0433\u0435\u043d\u0442\u043e\u0432")
})

test("falls back for unknown codes and unsupported locales", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("unknown.thing", "\u539f\u59cb\u540d\u79f0", "ru-RU"), "\u539f\u59cb\u540d\u79f0")
  assert.equal(getPermissionDisplayName("user.view", "\u539f\u59cb\u540d\u79f0", "fr-FR"), "\u539f\u59cb\u540d\u79f0")
  assert.equal(getPermissionGroupName("unknownGroup", "ru-RU"), "unknownGroup")
  assert.equal(getPermissionGroupName("user", "fr-FR"), "user")
})
