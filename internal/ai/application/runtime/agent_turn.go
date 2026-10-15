package runtime

import (
	"context"
	"fmt"
	"strings"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/toolx"
	"agent-desk/internal/pkg/utils"
	svc "agent-desk/internal/services"
)

type agentLoopTurn struct {
	RetrieverCount int
	RetrieveErr    error
	ResponsePolicy agentLoopResponsePolicy
	SystemPrompt   string
	UserPrompt     string
	HistoryCount   int
	AllowedTools   []string
	ToolPolicy     agentLoopToolPolicy
	Skills         map[int64]models.SkillDefinition
	Workflows      map[int64]svc.AgentRevisionWorkflowBinding
	MaxSteps       int
}

func (e *AgentLoopEngine) prepareTurn(ctx context.Context, req RunInput, snapshot *svc.AgentRevisionSnapshot) agentLoopTurn {
	maxSteps := req.AIAgent.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 4
	}
	if maxSteps > 8 {
		maxSteps = 8
	}
	knowledgeContext, retrieverCount, retrieveErr := e.retrieveKnowledge(ctx, req.AIAgent, req.UserMessage.Content)
	responsePolicy := evaluateAgentLoopResponsePolicy(req.AIAgent, knowledgeContext, retrieveErr)
	systemPrompt := buildAgentLoopSystemPrompt(req.AIAgent, len(utils.SplitInt64s(req.AIAgent.KnowledgeIDs)) > 0, knowledgeContext, retrieveErr)
	userPrompt, historyCount := e.buildUserPrompt(req)
	if knowledgeContext != "" {
		userPrompt += "\n\nKnowledge evidence:\n" + knowledgeContext
	}
	skills := svc.SkillDefinitionService.GetByIDs(utils.SplitInt64s(req.AIAgent.SkillIDs))
	workflows := make(map[int64]svc.AgentRevisionWorkflowBinding, len(snapshot.WorkflowBindings))
	allowedTools := agentLoopSafeBuiltinCodes()
	// TODO 这么实现我觉得不太好，最好是能够有个统一的能力目录
	catalog := []string{
		"- " + toolx.BuiltinConversationContext.Code + " | Builtin | Read current conversation and customer context | direct function: " + toolx.SanitizeToolFunctionName(toolx.BuiltinConversationContext.Code),
		"- " + toolx.BuiltinKnowledgeRetrieve.Code + " | Builtin | Retrieve linked knowledge base on demand | direct function: " + toolx.SanitizeToolFunctionName(toolx.BuiltinKnowledgeRetrieve.Code),
		"- " + toolx.GraphTriageServiceRequest.Code + " | Builtin | Analyze service request and generate handling advice | direct function: " + toolx.SanitizeToolFunctionName(toolx.GraphTriageServiceRequest.Code),
		"- " + toolx.GraphAnalyzeConversation.Code + " | Builtin | Analyze conversation intent and risk signals | direct function: " + toolx.SanitizeToolFunctionName(toolx.GraphAnalyzeConversation.Code),
		"- " + toolx.GraphPrepareTicketDraft.Code + " | Builtin | Generate ticket draft without writing | direct function: " + toolx.SanitizeToolFunctionName(toolx.GraphPrepareTicketDraft.Code),
	}
	for id, skill := range skills {
		if skill.Status != enums.StatusOk {
			continue
		}
		code := agentLoopSkillCode(id)
		allowedTools = append(allowedTools, code)
		catalog = append(catalog, fmt.Sprintf("- %s | Skill | %s | %s | direct function: %s", code, strings.TrimSpace(skill.Name), strings.TrimSpace(skill.Description), toolx.SanitizeToolFunctionName(code)))
	}
	for _, binding := range snapshot.WorkflowBindings {
		if binding.WorkflowVersionID <= 0 {
			continue
		}
		workflows[binding.WorkflowVersionID] = binding
		code := agentLoopWorkflowCode(binding.WorkflowVersionID)
		allowedTools = append(allowedTools, code)
		catalog = append(catalog, fmt.Sprintf("- %s | Workflow | %s | %s | direct function: %s", code, strings.TrimSpace(binding.ToolName), strings.TrimSpace(binding.TriggerInstruction), toolx.SanitizeToolFunctionName(code)))
	}
	mcpTools, _ := toolx.ParseAgentMCPToolsJSON(req.AIAgent.AllowedMCPTools)
	for _, tool := range mcpTools {
		if strings.TrimSpace(tool.ToolCode) == "" {
			continue
		}
		allowedTools = append(allowedTools, tool.ToolCode)
		catalog = append(catalog, fmt.Sprintf("- %s | MCP | %s | %s | direct function: %s", tool.ToolCode, tool.Title, tool.Description, toolx.SanitizeToolFunctionName(tool.ToolCode)))
	}
	systemPrompt += "\n\nAvailable capabilities:\n" + strings.Join(catalog, "\n")
	systemPrompt += "\n\nUse tool_search with an exact capability code only when needed. You decide whether to answer directly, activate a Skill, execute a Workflow, retrieve knowledge, or call MCP. A Skill activation returns instructions for this same run. Never invent a capability code. Never pass a capability code containing slashes as a tool function name; use the exact direct function name provided in the catalog or call tool_search with toolCode. For any requested internal action such as human handoff, call conversation_decision; its action is a structured proposal only, and the runtime performs the action. When the customer explicitly asks for human support, set action=handoff, handoffInitiator=customer, and handoffConfirmed=true; do not ask again. Use ask_handoff_confirmation only when you, not the customer, recommend an unconfirmed handoff, with handoffInitiator=agent and handoffConfirmed=false. Never claim a handoff, assignment, or queue entry succeeded in reply text."
	systemPrompt += "\n\nCRITICAL RESPONSE LANGUAGE REQUIREMENT: You must inspect the customer's language in their message. If the customer wrote in Russian, you MUST reply strictly in Russian. If the customer wrote in English, you MUST reply strictly in English. Do NOT default to Chinese. Under no circumstances should you reply in Chinese when the customer's query is in Russian or English."
	return agentLoopTurn{
		RetrieverCount: retrieverCount,
		RetrieveErr:    retrieveErr,
		ResponsePolicy: responsePolicy,
		SystemPrompt:   systemPrompt,
		UserPrompt:     userPrompt,
		HistoryCount:   historyCount,
		AllowedTools:   allowedTools,
		ToolPolicy:     parseAgentLoopToolPolicy(req.AIAgent.ToolPolicy),
		Skills:         skills,
		Workflows:      workflows,
		MaxSteps:       maxSteps,
	}
}
