const SEEDED_ROLE_LABELS: Record<string, Record<string, string>> = {
  "en-US": {
    super_admin: "Super admin",
    admin: "Admin",
    cs_team_leader: "Support team lead",
    cs_user: "Support agent",
  },
  "ru-RU": {
    super_admin: "Суперадминистратор",
    admin: "Администратор",
    cs_team_leader: "Руководитель команды поддержки",
    cs_user: "Оператор поддержки",
  },
}

export function getRoleDisplayName(
  code: string | undefined,
  fallbackName: string,
  locale: string
) {
  const labels = SEEDED_ROLE_LABELS[locale]
  if (!labels) {
    return fallbackName
  }
  const label = labels[code?.trim() ?? ""]
  return label || fallbackName
}