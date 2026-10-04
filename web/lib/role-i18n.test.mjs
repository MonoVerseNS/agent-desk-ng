import assert from "node:assert/strict"
import test from "node:test"
import ts from "typescript"
import { readFile } from "node:fs/promises"
import vm from "node:vm"

async function loadModule() {
  const source = await readFile(new URL("./role-i18n.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "role-i18n.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("localizes seeded role names to English by role code", async () => {
  const { getRoleDisplayName } = await loadModule()

  assert.equal(getRoleDisplayName("super_admin", "\u8d85\u7ea7\u7ba1\u7406\u5458", "en-US"), "Super admin")
  assert.equal(getRoleDisplayName("cs_team_leader", "\u5ba2\u670d\u7ec4\u957f", "en-US"), "Support team lead")
  assert.equal(getRoleDisplayName("cs_user", "\u5ba2\u670d", "en-US"), "Support agent")
})

test("keeps custom or Chinese role names unchanged", async () => {
  const { getRoleDisplayName } = await loadModule()

  assert.equal(getRoleDisplayName("custom", "Ops reviewer", "en-US"), "Ops reviewer")
  assert.equal(getRoleDisplayName("super_admin", "\u8d85\u7ea7\u7ba1\u7406\u5458", "zh-CN"), "\u8d85\u7ea7\u7ba1\u7406\u5458")
})

test("localizes seeded role names to Russian by role code", async () => {
  const { getRoleDisplayName } = await loadModule()

  assert.equal(
    getRoleDisplayName("super_admin", "\u8d85\u7ea7\u7ba1\u7406\u5458", "ru-RU"),
    "\u0421\u0443\u043f\u0435\u0440\u0430\u0434\u043c\u0438\u043d\u0438\u0441\u0442\u0440\u0430\u0442\u043e\u0440"
  )
  assert.equal(
    getRoleDisplayName("cs_team_leader", "\u5ba2\u670d\u7ec4\u957f", "ru-RU"),
    "\u0420\u0443\u043a\u043e\u0432\u043e\u0434\u0438\u0442\u0435\u043b\u044c \u043a\u043e\u043c\u0430\u043d\u0434\u044b \u043f\u043e\u0434\u0434\u0435\u0440\u0436\u043a\u0438"
  )
  assert.equal(getRoleDisplayName("cs_user", "\u5ba2\u670d", "ru-RU"), "\u041e\u043f\u0435\u0440\u0430\u0442\u043e\u0440 \u043f\u043e\u0434\u0434\u0435\u0440\u0436\u043a\u0438")
  assert.equal(getRoleDisplayName("custom", "Ops reviewer", "ru-RU"), "Ops reviewer")
  assert.equal(getRoleDisplayName("super_admin", "\u8d85\u7ea7\u7ba1\u7406\u5458", "fr-FR"), "\u8d85\u7ea7\u7ba1\u7406\u5458")
})
