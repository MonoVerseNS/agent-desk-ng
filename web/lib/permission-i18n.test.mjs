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
  assert.equal(getPermissionDisplayName("agentTeamSchedule.batchGenerate", "批量生成客服组排班", "en-US"), "Batch generate agent team schedules")
  assert.equal(getPermissionDisplayName("branding.view", "查看平台品牌配置", "en-US"), "View branding settings")
  assert.equal(getPermissionDisplayName("docPage.view", "查看文档中心页面", "en-US"), "View support docs")
  assert.equal(getPermissionGroupName("agentTeamSchedule", "en-US"), "Agent team schedules")
  assert.equal(getPermissionGroupName("branding", "en-US"), "Branding settings")
})

test("keeps original permission names for Chinese locale", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("user.view", "查看用户", "zh-CN"), "查看用户")
  assert.equal(getPermissionDisplayName("branding.view", "查看平台品牌配置", "zh-CN"), "查看平台品牌配置")
  assert.equal(getPermissionGroupName("agentTeam", "zh-CN"), "agentTeam")
})

test("localizes seeded permission display names to Russian", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(
    getPermissionDisplayName("user.assignRole", "分配用户角色", "ru-RU"),
    "Назначение ролей пользователям"
  )
  assert.equal(getPermissionDisplayName("user.view", "查看用户", "ru-RU"), "Просмотр пользователей")
  assert.equal(getPermissionDisplayName("branding.view", "查看平台品牌配置", "ru-RU"), "Просмотр настроек брендинга")
  assert.equal(getPermissionDisplayName("docPage.view", "查看文档中心页面", "ru-RU"), "Просмотр страниц документации")
  assert.equal(getPermissionDisplayName("supportConfig.update", "更新支持中心配置", "ru-RU"), "Изменение настроек центра поддержки")
  assert.equal(
    getPermissionDisplayName("agentTeamSchedule.batchGenerate", "批量生成客服组排班", "ru-RU"),
    "Массовое создание расписаний команд операторов"
  )
  assert.equal(getPermissionGroupName("agentTeamSchedule", "ru-RU"), "Расписаний команд операторов")
  assert.equal(getPermissionGroupName("aiAgent", "ru-RU"), "ИИ-агентов")
  assert.equal(getPermissionGroupName("branding", "ru-RU"), "Настроек брендинга")
})

test("falls back for unknown codes and unsupported locales", async () => {
  const { getPermissionDisplayName, getPermissionGroupName } = await loadModule()

  assert.equal(getPermissionDisplayName("unknown.thing", "\u539f\u59cb\u540d\u79f0", "ru-RU"), "\u539f\u59cb\u540d\u79f0")
  assert.equal(getPermissionDisplayName("user.view", "\u539f\u59cb\u540d\u79f0", "fr-FR"), "\u539f\u59cb\u540d\u79f0")
  assert.equal(getPermissionGroupName("unknownGroup", "ru-RU"), "unknownGroup")
  assert.equal(getPermissionGroupName("user", "fr-FR"), "user")
})
