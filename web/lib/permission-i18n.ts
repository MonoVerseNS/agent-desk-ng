type PermissionVocabulary = {
  actions: Record<string, string>
  resources: Record<string, { singular: string; plural: string }>
  nameOverrides: Record<string, string>
  /** Labels that must not be re-capitalized, e.g. acronyms. */
  preserveCase?: readonly string[]
}

const EN_PERMISSION_VOCABULARY: PermissionVocabulary = {
  actions: {
    view: "View",
    create: "Create",
    update: "Update",
    delete: "Delete",
    assignRole: "Assign roles to",
    assignPermission: "Assign permissions to",
    sync: "Sync",
    revoke: "Revoke",
    assign: "Assign",
    transfer: "Transfer",
    close: "Close",
    send: "Send",
    tag: "Manage tags for",
    handover: "Handle handoffs for",
    recycle: "Recycle",
    linkCustomer: "Link customers to",
    changeStatus: "Change status for",
    progress: "Update progress for",
    resetUserTokenSecret: "Reset user token secret for",
    updateStatus: "Update status for",
    config: "Configure service rules for",
    batchGenerate: "Batch generate",
    call: "Call",
  },
  resources: {
    user: { singular: "user", plural: "users" },
    role: { singular: "role", plural: "roles" },
    permission: { singular: "permission", plural: "permissions" },
    session: { singular: "session", plural: "sessions" },
    conversation: { singular: "conversation", plural: "conversations" },
    ticket: { singular: "ticket", plural: "tickets" },
    notification: { singular: "notification", plural: "notifications" },
    quickReply: { singular: "quick reply", plural: "quick replies" },
    tag: { singular: "tag", plural: "tags" },
    company: { singular: "company", plural: "companies" },
    channel: { singular: "channel", plural: "channels" },
    wxworkOutbox: { singular: "WeCom outbox record", plural: "WeCom outbox records" },
    customer: { singular: "customer", plural: "customers" },
    agent: { singular: "agent", plural: "agents" },
    agentTeam: { singular: "agent team", plural: "agent teams" },
    agentTeamSchedule: { singular: "agent team schedule", plural: "agent team schedules" },
    asset: { singular: "file asset", plural: "file assets" },
    aiAgent: { singular: "AI Agent", plural: "AI Agents" },
    aiConfig: { singular: "AI configuration", plural: "AI configurations" },
    knowledgeBase: { singular: "knowledge base", plural: "knowledge bases" },
    knowledgeDocument: { singular: "knowledge document", plural: "knowledge documents" },
    knowledgeFAQ: { singular: "knowledge FAQ", plural: "knowledge FAQs" },
    skillDefinition: { singular: "Skill definition", plural: "Skill definitions" },
    mcp: { singular: "MCP tool", plural: "MCP tools" },
  },
  nameOverrides: {
    "user.assignRole": "Assign user roles",
    "role.assignPermission": "Assign role permissions",
    "session.revoke": "Revoke sessions",
    "conversation.linkCustomer": "Link conversation customer",
    "ticket.changeStatus": "Change ticket status",
    "ticket.progress": "Update ticket progress",
    "channel.resetUserTokenSecret": "Reset channel user token secret",
    "agent.config": "Configure agent service rules",
    "agentTeamSchedule.batchGenerate": "Batch generate agent team schedules",
    "wxworkOutbox.update": "Handle WeCom outbox records",
    "mcp.view": "View MCP debug information",
    "mcp.call": "Call MCP tools",
  },
  preserveCase: ["AI Agents", "MCP tools"],
}

/**
 * Russian permission labels are built from genitive-plural resource names so
 * the action noun and the resource agree grammatically.
 */
const RU_PERMISSION_VOCABULARY: PermissionVocabulary = {
  actions: {
    view: "Просмотр",
    create: "Создание",
    update: "Изменение",
    delete: "Удаление",
    assignRole: "Назначение ролей",
    assignPermission: "Назначение разрешений",
    sync: "Синхронизация",
    revoke: "Отзыв",
    assign: "Назначение",
    transfer: "Передача",
    close: "Закрытие",
    send: "Отправка",
    tag: "Управление тегами",
    handover: "Обработка передач",
    recycle: "Корзина",
    linkCustomer: "Связывание клиентов",
    changeStatus: "Изменение статуса",
    progress: "Обновление хода работ",
    resetUserTokenSecret: "Сброс секрета токена",
    updateStatus: "Изменение статуса",
    config: "Настройка правил обслуживания",
    batchGenerate: "Массовое создание",
    call: "Вызов",
  },
  resources: {
    user: { singular: "пользователь", plural: "пользователей" },
    role: { singular: "роль", plural: "ролей" },
    permission: { singular: "разрешение", plural: "разрешений" },
    session: { singular: "сессия", plural: "сессий" },
    conversation: { singular: "диалог", plural: "диалогов" },
    ticket: { singular: "обращение", plural: "обращений" },
    notification: { singular: "уведомление", plural: "уведомлений" },
    quickReply: { singular: "быстрый ответ", plural: "быстрых ответов" },
    tag: { singular: "тег", plural: "тегов" },
    company: { singular: "компания", plural: "компаний" },
    channel: { singular: "канал", plural: "каналов" },
    wxworkOutbox: { singular: "запись исходящих WeCom", plural: "записей исходящих WeCom" },
    customer: { singular: "клиент", plural: "клиентов" },
    agent: { singular: "оператор", plural: "операторов" },
    agentTeam: { singular: "команда операторов", plural: "команд операторов" },
    agentTeamSchedule: { singular: "расписание команды операторов", plural: "расписаний команд операторов" },
    asset: { singular: "файл", plural: "файлов" },
    aiAgent: { singular: "ИИ-агент", plural: "ИИ-агентов" },
    aiConfig: { singular: "конфигурация ИИ", plural: "конфигураций ИИ" },
    knowledgeBase: { singular: "база знаний", plural: "баз знаний" },
    knowledgeDocument: { singular: "документ базы знаний", plural: "документов базы знаний" },
    knowledgeFAQ: { singular: "вопрос FAQ", plural: "вопросов FAQ" },
    skillDefinition: { singular: "определение навыка", plural: "определений навыков" },
    mcp: { singular: "инструмент MCP", plural: "инструментов MCP" },
  },
  nameOverrides: {
    "user.assignRole": "Назначение ролей пользователям",
    "role.assignPermission": "Назначение разрешений роли",
    "session.revoke": "Отзыв сессий",
    "conversation.linkCustomer": "Связывание клиента с диалогом",
    "ticket.changeStatus": "Изменение статуса обращения",
    "ticket.progress": "Обновление хода работ по обращению",
    "channel.resetUserTokenSecret": "Сброс секрета токена пользователя канала",
    "agent.config": "Настройка правил обслуживания оператора",
    "agentTeamSchedule.batchGenerate": "Массовое создание расписаний команд операторов",
    "wxworkOutbox.update": "Обработка исходящих записей WeCom",
    "mcp.view": "Просмотр отладочной информации MCP",
    "mcp.call": "Вызов инструментов MCP",
  },
}

const PERMISSION_VOCABULARIES: Record<string, PermissionVocabulary> = {
  "en-US": EN_PERMISSION_VOCABULARY,
  "ru-RU": RU_PERMISSION_VOCABULARY,
}

export function getPermissionDisplayName(
  code: string | undefined,
  fallbackName: string,
  locale: string
) {
  const vocabulary = PERMISSION_VOCABULARIES[locale]
  if (!vocabulary) {
    return fallbackName
  }
  const normalizedCode = code?.trim() ?? ""
  if (!normalizedCode) {
    return fallbackName
  }
  const override = vocabulary.nameOverrides[normalizedCode]
  if (override) {
    return override
  }
  const [resourceKey, actionKey] = normalizedCode.split(".")
  const resource = vocabulary.resources[resourceKey]
  const action = vocabulary.actions[actionKey]
  if (!resource || !action) {
    return fallbackName
  }
  return `${action} ${resource.plural}`
}

export function getPermissionGroupName(groupName: string | undefined, locale: string) {
  const normalizedGroupName = groupName?.trim() ?? ""
  const vocabulary = PERMISSION_VOCABULARIES[locale]
  if (!vocabulary) {
    return normalizedGroupName
  }
  const resource = vocabulary.resources[normalizedGroupName]
  if (!resource) {
    return normalizedGroupName
  }
  return sentenceCase(resource.plural, vocabulary.preserveCase)
}

function sentenceCase(value: string, preserveCase?: readonly string[]) {
  if (preserveCase?.includes(value)) {
    return value
  }
  return value.charAt(0).toUpperCase() + value.slice(1)
}