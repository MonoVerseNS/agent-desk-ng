import test from "node:test"
import assert from "node:assert/strict"
import { getAgentActivityLabel, formatOperatorToolName } from "./agent-activity-label.ts"

// Mock translator that mimics t(key, params)
function mockT(key, params = {}) {
  const dictionary = {
    "agentActivity.thinking": "AI-ассистент думает...",
    "agentActivity.searchingKnowledge": "Поиск в базе знаний...",
    "agentActivity.callingTool": `Выполнение действия: ${params.name || "..."}...`,
    "agentActivity.runningWorkflow": `Выполнение сценария: ${params.name || "..."}...`,
    "agentActivity.default": "AI-ассистент обрабатывает запрос...",
    "agentActivity.customerThinking": "AI-ассистент думает...",
    "agentActivity.customerSearching": "Поиск информации...",
    "agentActivity.customerProcessing": "AI-ассистент готовит ответ...",
    "agentActivity.customerDefault": "AI-ассистент готовит ответ...",
    "agentActivity.toolAnalyzeConversation": "Анализ диалога",
    "agentActivity.toolTriageServiceRequest": "Маршрутизация обращения",
    "agentActivity.toolCreateTicket": "Создание обращения",
    "agentActivity.toolHandoff": "Перевод на оператора",
  }
  return dictionary[key] ?? key
}

test("formatOperatorToolName formats known graph tools to localized human titles", () => {
  assert.equal(formatOperatorToolName("graph_analyze_conversation", mockT), "Анализ диалога")
  assert.equal(formatOperatorToolName("graph/analyze_conversation", mockT), "Анализ диалога")
  assert.equal(formatOperatorToolName("analyze_conversation", mockT), "Анализ диалога")
  assert.equal(formatOperatorToolName("graph_triage_service_request", mockT), "Маршрутизация обращения")
  assert.equal(formatOperatorToolName("graph_create_ticket", mockT), "Создание обращения")
  assert.equal(formatOperatorToolName("graph_handoff", mockT), "Перевод на оператора")
})

test("formatOperatorToolName formats unknown tools by stripping prefixes and converting underscores", () => {
  assert.equal(formatOperatorToolName("mcp_fetch_order_status", mockT), "mcp fetch order status")
  assert.equal(formatOperatorToolName("workflow_refund_check", mockT), "refund check")
  assert.equal(formatOperatorToolName("skill_order_lookup", mockT), "order lookup")
  assert.equal(formatOperatorToolName("", mockT), "...")
  assert.equal(formatOperatorToolName(undefined, mockT), "...")
})

test("customer mode (isCustomer=true) never exposes tool names or technical system jargon", () => {
  // Calling tool with internal graph_analyze_conversation
  const toolLabel = getAgentActivityLabel(
    { stepCode: "calling_tool", toolName: "graph_analyze_conversation", isCustomer: true },
    mockT
  )
  assert.equal(toolLabel, "AI-ассистент готовит ответ...")
  assert.ok(!toolLabel.includes("graph_analyze_conversation"))
  assert.ok(!toolLabel.includes("Выполнение действия"))

  // Running workflow
  const workflowLabel = getAgentActivityLabel(
    { stepCode: "running_workflow", toolName: "workflow_triage_request", isCustomer: true },
    mockT
  )
  assert.equal(workflowLabel, "AI-ассистент готовит ответ...")
  assert.ok(!workflowLabel.includes("workflow_triage_request"))

  // Searching knowledge
  const searchLabel = getAgentActivityLabel(
    { stepCode: "searching_knowledge", isCustomer: true },
    mockT
  )
  assert.equal(searchLabel, "Поиск информации...")
  assert.ok(!searchLabel.includes("базе знаний"))

  // Thinking
  const thinkingLabel = getAgentActivityLabel(
    { stepCode: "thinking", isCustomer: true },
    mockT
  )
  assert.equal(thinkingLabel, "AI-ассистент думает...")

  // Defaults to customer mode if isCustomer is omitted
  const defaultCustomerLabel = getAgentActivityLabel(
    { stepCode: "calling_tool", toolName: "graph_analyze_conversation" },
    mockT
  )
  assert.equal(defaultCustomerLabel, "AI-ассистент готовит ответ...")
})

test("operator mode (isCustomer=false) provides clear formatted context to operators", () => {
  const toolLabel = getAgentActivityLabel(
    { stepCode: "calling_tool", toolName: "graph_analyze_conversation", isCustomer: false },
    mockT
  )
  assert.equal(toolLabel, "Выполнение действия: Анализ диалога...")

  const workflowLabel = getAgentActivityLabel(
    { stepCode: "running_workflow", toolName: "workflow_sync_data", isCustomer: false },
    mockT
  )
  assert.equal(workflowLabel, "Выполнение сценария: sync data...")

  const searchLabel = getAgentActivityLabel(
    { stepCode: "searching_knowledge", isCustomer: false },
    mockT
  )
  assert.equal(searchLabel, "Поиск в базе знаний...")

  const thinkingLabel = getAgentActivityLabel(
    { stepCode: "thinking", isCustomer: false },
    mockT
  )
  assert.equal(thinkingLabel, "AI-ассистент думает...")
})
