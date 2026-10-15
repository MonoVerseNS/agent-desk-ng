export type AgentActivityLabelOptions = {
  stepCode: string
  toolName?: string
  isCustomer?: boolean
}

export type TranslateFn = (key: string, values?: Record<string, string>) => string

export function formatOperatorToolName(toolName?: string, t?: TranslateFn): string {
  if (!toolName) return "..."

  const normalized = toolName.trim()
  if (
    normalized === "graph_analyze_conversation" ||
    normalized === "analyze_conversation" ||
    normalized === "graph/analyze_conversation"
  ) {
    return t ? t("agentActivity.toolAnalyzeConversation") : "Analyze conversation"
  }
  if (
    normalized === "graph_triage_service_request" ||
    normalized === "triage_service_request" ||
    normalized === "graph/triage_service_request"
  ) {
    return t ? t("agentActivity.toolTriageServiceRequest") : "Triage service request"
  }
  if (
    normalized === "graph_create_ticket" ||
    normalized === "create_ticket" ||
    normalized === "graph/create_ticket"
  ) {
    return t ? t("agentActivity.toolCreateTicket") : "Create ticket"
  }
  if (
    normalized === "graph_handoff" ||
    normalized === "handoff" ||
    normalized === "graph/handoff"
  ) {
    return t ? t("agentActivity.toolHandoff") : "Human handoff"
  }

  // Remove common prefixes: graph_, skill_, workflow_, builtin_, or graph/
  const clean = normalized
    .replace(/^(graph_|skill_|workflow_|builtin_|graph\/)/, "")
    .replace(/_/g, " ")
    .trim()

  return clean || normalized
}

export function getAgentActivityLabel(
  options: AgentActivityLabelOptions,
  t: TranslateFn
): string {
  const { stepCode, toolName, isCustomer = true } = options

  // For customers (end-users in support chat):
  // Never expose internal system details, technical function names, or tool execution steps.
  if (isCustomer) {
    if (stepCode === "thinking") {
      return t("agentActivity.customerThinking")
    }
    if (stepCode === "searching_knowledge") {
      return t("agentActivity.customerSearching")
    }
    // For calling_tool, running_workflow, or any other internal action
    return t("agentActivity.customerProcessing")
  }

  // For operators (in dashboard workspace):
  // Surface informative step and formatted tool name
  if (stepCode === "thinking") {
    return t("agentActivity.thinking")
  }
  if (stepCode === "searching_knowledge") {
    return t("agentActivity.searchingKnowledge")
  }
  if (stepCode === "calling_tool") {
    const formatted = formatOperatorToolName(toolName, t)
    return t("agentActivity.callingTool", { name: formatted })
  }
  if (stepCode === "running_workflow") {
    const formatted = formatOperatorToolName(toolName, t)
    return t("agentActivity.runningWorkflow", { name: formatted })
  }

  return t("agentActivity.default")
}
