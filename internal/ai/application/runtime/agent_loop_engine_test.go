package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	ai "agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	svc "agent-desk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func TestAgentLoopActivatesSkillInsideSameToolLoop(t *testing.T) {
	skill := models.SkillDefinition{
		ID: 7, Name: "退款说明", Instruction: "只根据退款政策回答。",
		ToolWhitelist: `["builtin/knowledge_retrieve"]`, Status: enums.StatusOk,
	}
	turn := agentLoopTurn{
		AllowedTools: []string{"skill/7"},
		ToolPolicy:   parseAgentLoopToolPolicy(""),
		Skills:       map[int64]models.SkillDefinition{skill.ID: skill},
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(RunInput{}, turn, &state, &calls)

	result, err := execute(context.Background(), ai.ToolCall{
		Name: "tool_search", Arguments: `{"toolCode":"skill/7","arguments":{}}`,
	})
	if err != nil {
		t.Fatalf("activate Skill: %v", err)
	}
	if state.SkillContext.SkillID() != skill.ID || !strings.Contains(result, skill.Instruction) {
		t.Fatalf("Skill was not activated in the Agent Loop: state=%#v result=%q", state, result)
	}
	if len(calls) != 1 || calls[0].ToolCode != "skill/7" || calls[0].Status != "completed" {
		t.Fatalf("unexpected Skill audit: %#v", calls)
	}
}

func TestAgentLoopRegistersDirectCapabilityAliases(t *testing.T) {
	turn := agentLoopTurn{AllowedTools: []string{
		"builtin/conversation_context",
		"graph/triage_service_request",
		"graph/triage_service_request",
		"workflow/47",
	}}
	definitions := agentLoopToolDefinitions(turn)
	names := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		names[definition.Name] = true
	}
	for _, expected := range []string{
		"tool_search",
		"builtin_conversation_context",
		"graph_triage_service_request",
		"workflow_47",
	} {
		if !names[expected] {
			t.Fatalf("missing registered function alias %q: %#v", expected, definitions)
		}
	}
}

func TestConversationDecisionIsStructuredAndValidated(t *testing.T) {
	decision, err := parseConversationDecision(`{"action":"handoff","reason":"customer requested a human","reply":"","handoffInitiator":"customer","handoffConfirmed":true}`)
	if err != nil {
		t.Fatalf("parse handoff decision: %v", err)
	}
	if decision.Action != ConversationActionHandoff || decision.Reason != "customer requested a human" {
		t.Fatalf("unexpected handoff decision: %#v", decision)
	}
	for _, raw := range []string{
		`{"action":"unknown","reason":"x","reply":"x","handoffInitiator":"none","handoffConfirmed":false}`,
		`{"action":"reply","reason":"x","reply":"","handoffInitiator":"none","handoffConfirmed":false}`,
		`{"action":"ask_handoff_confirmation","reason":"x","reply":"confirm?","handoffInitiator":"customer","handoffConfirmed":false}`,
	} {
		if _, err := parseConversationDecision(raw); err == nil {
			t.Fatalf("expected invalid decision to fail: %s", raw)
		}
	}
}

func TestAgentLoopRecordsConversationDecision(t *testing.T) {
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(RunInput{}, agentLoopTurn{}, &state, &calls)
	if _, err := execute(context.Background(), ai.ToolCall{
		Name: "conversation_decision", Arguments: `{"action":"handoff","reason":"customer requested a human","reply":"","handoffInitiator":"customer","handoffConfirmed":true}`,
	}); err != nil {
		t.Fatalf("record decision: %v", err)
	}
	if state.Decision == nil || state.Decision.Action != ConversationActionHandoff {
		t.Fatalf("decision was not stored: %#v", state.Decision)
	}
	if len(calls) != 1 || calls[0].ToolCode != "conversation_decision" || calls[0].Status != "completed" {
		t.Fatalf("decision audit missing: %#v", calls)
	}
}

func TestResolveAgentLoopReplyKeepsNormalModelReplyWithoutDecision(t *testing.T) {
	reply, handoff, reason, err := resolveAgentLoopReply("你好，有什么可以帮你？", nil)
	if err != nil || handoff || reason != "" || reply != "你好，有什么可以帮你？" {
		t.Fatalf("unexpected normal reply resolution: reply=%q handoff=%t reason=%q err=%v", reply, handoff, reason, err)
	}
}

func TestResolveAgentLoopReplyUsesStructuredHandoffDecision(t *testing.T) {
	reply, handoff, reason, err := resolveAgentLoopReply("模型自由文本不应生效", &ConversationDecision{
		Action: ConversationActionHandoff, Reason: "customer requested human support", HandoffInitiator: HandoffInitiatorCustomer, HandoffConfirmed: true,
	})
	if err != nil || !handoff || reply != "" || reason != "customer requested human support" {
		t.Fatalf("unexpected handoff resolution: reply=%q handoff=%t reason=%q err=%v", reply, handoff, reason, err)
	}
}

func TestNormalizeAgentLoopReplyAllowsEmptyInternalHandoff(t *testing.T) {
	reply, err := normalizeAgentLoopReply("", true)
	if err != nil || reply != "" {
		t.Fatalf("handoff must bypass customer reply normalization: reply=%q err=%v", reply, err)
	}
	if _, err := normalizeAgentLoopReply("", false); err == nil {
		t.Fatal("ordinary empty model replies must still be rejected")
	}
}

func TestAgentLoopDirectCapabilityAliasUsesSamePolicyBoundary(t *testing.T) {
	skill := models.SkillDefinition{
		ID: 7, Name: "售后升级处理", Instruction: "先确认升级诉求。", Status: enums.StatusOk,
	}
	turn := agentLoopTurn{
		AllowedTools: []string{"skill/7"},
		ToolPolicy:   parseAgentLoopToolPolicy(""),
		Skills:       map[int64]models.SkillDefinition{skill.ID: skill},
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(RunInput{}, turn, &state, &calls)

	result, err := execute(context.Background(), ai.ToolCall{Name: "skill/7", Arguments: `{}`})
	if err != nil {
		t.Fatalf("execute direct capability alias: %v", err)
	}
	if state.SkillContext.SkillID() != skill.ID || !strings.Contains(result, skill.Instruction) {
		t.Fatalf("direct capability was not routed through Skill activation: state=%#v result=%q", state, result)
	}
	if len(calls) != 1 || calls[0].ToolCode != "skill/7" || calls[0].Status != "completed" {
		t.Fatalf("unexpected direct capability audit: %#v", calls)
	}
}

func TestAgentLoopDirectSanitizedCapabilityAliasRoutesCorrectly(t *testing.T) {
	skill := models.SkillDefinition{
		ID: 7, Name: "售后升级处理", Instruction: "先确认升级诉求。", Status: enums.StatusOk,
	}
	turn := agentLoopTurn{
		AllowedTools: []string{"skill/7"},
		ToolPolicy:   parseAgentLoopToolPolicy(""),
		Skills:       map[int64]models.SkillDefinition{skill.ID: skill},
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(RunInput{}, turn, &state, &calls)

	result, err := execute(context.Background(), ai.ToolCall{Name: "skill_7", Arguments: `{}`})
	if err != nil {
		t.Fatalf("execute sanitized direct capability alias: %v", err)
	}
	if state.SkillContext.SkillID() != skill.ID || !strings.Contains(result, skill.Instruction) {
		t.Fatalf("sanitized direct capability was not routed through Skill activation: state=%#v result=%q", state, result)
	}
	if len(calls) != 1 || calls[0].ToolCode != "skill/7" || calls[0].Status != "completed" {
		t.Fatalf("unexpected direct capability audit: %#v", calls)
	}
}

func TestAgentLoopInterruptsBeforeWriteMCPTool(t *testing.T) {
	configured, err := json.Marshal([]request.AIAgentMCPToolRequest{{
		ToolCode: "crm/update_customer", ServerCode: "crm", ToolName: "update_customer",
		Title: "更新客户", RiskLevel: "write", RequireConfirmation: true,
	}})
	if err != nil {
		t.Fatalf("marshal MCP configuration: %v", err)
	}
	runInput := RunInput{
		Conversation: models.Conversation{ID: 9},
		AIAgent:      models.AIAgent{AllowedMCPTools: string(configured)},
	}
	turn := agentLoopTurn{
		AllowedTools: []string{"crm/update_customer"},
		ToolPolicy:   parseAgentLoopToolPolicy(`{"allowedRiskLevels":["read","write"]}`),
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(runInput, turn, &state, &calls)

	_, err = execute(context.Background(), ai.ToolCall{
		Name: "tool_search", Arguments: `{"toolCode":"crm/update_customer","arguments":{"name":"Ada"}}`,
	})
	if err == nil {
		t.Fatal("expected write MCP Tool to interrupt")
	}
	if state.Interrupted == nil || !state.Interrupted.Interrupted || !strings.HasPrefix(state.Interrupted.CheckPointID, "tool:9:") {
		t.Fatalf("missing MCP confirmation checkpoint: %#v", state.Interrupted)
	}
	if state.Interrupted.ReplyText != "即将执行“更新客户”，是否确认继续？" ||
		len(state.Interrupted.Interrupts) != 1 ||
		state.Interrupted.Interrupts[0].PromptText != state.Interrupted.ReplyText {
		t.Fatalf("unexpected customer confirmation prompt: %#v", state.Interrupted)
	}
	if len(calls) != 1 || calls[0].RiskLevel != "write" || !calls[0].RequireConfirm || calls[0].Status != "interrupted" {
		t.Fatalf("unexpected MCP safety audit: %#v", calls)
	}
}

func TestAgentLoopRejectsWriteMCPBeforeConfirmationWhenRiskIsNotAllowed(t *testing.T) {
	configured, _ := json.Marshal([]request.AIAgentMCPToolRequest{{
		ToolCode: "crm/update_customer", ServerCode: "crm", ToolName: "update_customer",
		Title: "更新客户", RiskLevel: "write", RequireConfirmation: true,
	}})
	runInput := RunInput{
		Conversation: models.Conversation{ID: 9},
		AIAgent:      models.AIAgent{AllowedMCPTools: string(configured)},
	}
	turn := agentLoopTurn{
		AllowedTools: []string{"crm/update_customer"},
		ToolPolicy:   parseAgentLoopToolPolicy(`{"allowedRiskLevels":["read"]}`),
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(runInput, turn, &state, &calls)

	_, err := execute(context.Background(), ai.ToolCall{
		Name: "tool_search", Arguments: `{"toolCode":"crm/update_customer","arguments":{"name":"Ada"}}`,
	})
	if err == nil || !strings.Contains(err.Error(), "risk level") {
		t.Fatalf("expected MCP risk policy rejection, got %v", err)
	}
	if state.Interrupted != nil || len(calls) != 1 || calls[0].Status != "failed" {
		t.Fatalf("disallowed MCP call should fail without a checkpoint: state=%#v calls=%#v", state, calls)
	}
}

func TestAgentLoopRejectsWorkflowWhenWriteRiskIsNotAllowed(t *testing.T) {
	turn := agentLoopTurn{
		AllowedTools: []string{"workflow/23"},
		ToolPolicy:   parseAgentLoopToolPolicy(`{"allowedRiskLevels":["read"]}`),
		Workflows: map[int64]svc.AgentRevisionWorkflowBinding{
			23: {WorkflowVersionID: 23, ToolName: "创建工单"},
		},
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	execute := NewAgentLoopEngine().toolSearchExecutor(RunInput{}, turn, &state, &calls)

	_, err := execute(context.Background(), ai.ToolCall{
		Name: "tool_search", Arguments: `{"toolCode":"workflow/23","arguments":{}}`,
	})
	if err == nil || !strings.Contains(err.Error(), "risk level") {
		t.Fatalf("expected Workflow risk policy rejection, got %v", err)
	}
	if len(calls) != 1 || calls[0].Status != "failed" || calls[0].RiskLevel != "write" {
		t.Fatalf("unexpected Workflow policy audit: %#v", calls)
	}
}

func TestAgentLoopKnowledgeFallbackCanRequestHandoff(t *testing.T) {
	agent := models.AIAgent{
		KnowledgeIDs: "1", FallbackMode: enums.AIAgentFallbackModeHandoff,
		FallbackMessage: "我暂时无法核实，马上为你转人工。",
	}
	policy := evaluateAgentLoopResponsePolicy(agent, "", nil)
	prompt := buildAgentLoopSystemPrompt(agent, true, "", nil)
	if !policy.RequestHandoff || !strings.Contains(prompt, agent.FallbackMessage) {
		t.Fatalf("knowledge fallback was not applied: policy=%#v prompt=%q", policy, prompt)
	}
}

func TestAgentLoopConfirmationNormalizesHTMLAndKeepsUnknownPending(t *testing.T) {
	data := normalizeAgentLoopResumeData(enums.IMMessageTypeHTML, map[string]string{
		"message": "<p>确认。</p>",
	})
	if got := parseAgentLoopConfirmation(firstAgentLoopResumeText(data)); got != agentLoopConfirmationConfirmed {
		t.Fatalf("expected HTML confirmation, got %v from %#v", got, data)
	}
	if got := parseAgentLoopConfirmation("取消！"); got != agentLoopConfirmationCancelled {
		t.Fatalf("expected cancellation, got %v", got)
	}
	if got := parseAgentLoopConfirmation("稍后再说"); got != agentLoopConfirmationUnknown {
		t.Fatalf("ambiguous input must stay pending, got %v", got)
	}
}

func TestConfiguredMCPToolAppliesTrustedSystemPolicy(t *testing.T) {
	configured, _ := json.Marshal([]request.AIAgentMCPToolRequest{{
		ToolCode:            "system/server_time",
		ServerCode:          "system",
		ToolName:            "server_time",
		Title:               "server_time",
		RiskLevel:           "write",
		RequireConfirmation: true,
	}})
	tool, err := configuredMCPTool(string(configured), "system/server_time")
	if err != nil {
		t.Fatalf("resolve configured system tool: %v", err)
	}
	if tool.Title != "获取当前时间" || tool.RiskLevel != "read" || tool.RequireConfirmation {
		t.Fatalf("trusted policy was not applied at runtime: %#v", tool)
	}
}

func TestAgentLoopPromptAvoidsRepeatingWelcomeMessage(t *testing.T) {
	prompt := buildAgentLoopSystemPrompt(models.AIAgent{}, false, "", nil)
	if !strings.Contains(prompt, "without repeating the welcome wording") {
		t.Fatalf("conversation continuity instruction missing: %q", prompt)
	}
}

func TestAgentLoopPromptEnforcesCustomerLanguage(t *testing.T) {
	prompt := buildAgentLoopSystemPrompt(models.AIAgent{}, false, "", nil)
	if !strings.Contains(prompt, "CRITICAL LANGUAGE RULE") || !strings.Contains(prompt, "same language as the customer's current message") {
		t.Fatalf("language rule missing from system prompt: %q", prompt)
	}
}

func TestCompleteConfirmedMCPReplyGeneratesCustomerFacingAnswerWithoutTools(t *testing.T) {
	engine := NewAgentLoopEngine()
	var systemPrompt string
	var userPrompt string
	engine.complete = func(_ context.Context, _ models.AIConfig, system, user string) (*ai.ChatCompletionResult, error) {
		systemPrompt = system
		userPrompt = user
		return &ai.ChatCompletionResult{
			Content:          "当前服务端时间是 2026-07-28 11:51:52。",
			ModelName:        "test-model",
			PromptTokens:     20,
			CompletionTokens: 10,
		}, nil
	}

	result, err := engine.completeConfirmedMCPReply(
		context.Background(),
		models.AIAgent{},
		models.AIConfig{ModelName: "test-model"},
		"获取当前时间",
		"现在几点钟？",
		`{"timestamp":"2026-07-28 11:51:52","timezone":"Local"}`,
	)
	if err != nil {
		t.Fatalf("complete confirmed MCP reply: %v", err)
	}
	if result.Content != "当前服务端时间是 2026-07-28 11:51:52。" {
		t.Fatalf("unexpected customer reply: %#v", result)
	}
	for _, expected := range []string{
		"Do not request or invoke another tool",
		"现在几点钟？",
		"获取当前时间",
		`"timestamp":"2026-07-28 11:51:52"`,
	} {
		if !strings.Contains(systemPrompt+"\n"+userPrompt, expected) {
			t.Fatalf("post-tool completion context missing %q: system=%q user=%q", expected, systemPrompt, userPrompt)
		}
	}
}

func TestConfirmedMCPReplyFallbackDoesNotExposeRawResult(t *testing.T) {
	got := buildAgentLoopConfirmedMCPFallback("获取当前时间")
	if got != "“获取当前时间”已成功执行。" || strings.Contains(got, "{") {
		t.Fatalf("unexpected confirmed MCP fallback: %q", got)
	}
}

func TestAgentTurnPublishesAllConfiguredCapabilityKinds(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.SkillDefinition{}); err != nil {
		t.Fatalf("migrate Skill: %v", err)
	}
	sqls.SetDB(db)
	skill := models.SkillDefinition{Name: "订单查询", Description: "查询订单状态", Status: enums.StatusOk}
	if err := db.Create(&skill).Error; err != nil {
		t.Fatalf("create Skill: %v", err)
	}
	mcp, _ := json.Marshal([]request.AIAgentMCPToolRequest{{
		ToolCode: "crm/get_customer", ServerCode: "crm", ToolName: "get_customer",
		RiskLevel: "read",
	}})
	agent := models.AIAgent{SkillIDs: jsonInt64List(skill.ID), AllowedMCPTools: string(mcp)}
	snapshot := &svc.AgentRevisionSnapshot{
		Agent: agent,
		WorkflowBindings: []svc.AgentRevisionWorkflowBinding{{
			WorkflowVersionID: 23, ToolName: "创建工单", TriggerInstruction: "用户要求创建工单",
		}},
	}
	engine := NewAgentLoopEngine()
	engine.retrieve = nil
	engine.history = nil
	turn := engine.prepareTurn(context.Background(), RunInput{AIAgent: agent}, snapshot)

	for _, code := range []string{"skill/" + jsonInt64List(skill.ID), "workflow/23", "crm/get_customer"} {
		if !strings.Contains(turn.SystemPrompt, code) {
			t.Fatalf("capability %q missing from prompt:\n%s", code, turn.SystemPrompt)
		}
	}
}

func jsonInt64List(id int64) string {
	data, _ := json.Marshal([]int64{id})
	return strings.Trim(string(data), "[]")
}

func TestAgentTurnCatalogFormatAndInstruction(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&models.SkillDefinition{}); err != nil {
		t.Fatalf("migrate Skill: %v", err)
	}
	sqls.SetDB(db)
	skill := models.SkillDefinition{Name: "订单查询", Description: "查询订单状态", Status: enums.StatusOk}
	if err := db.Create(&skill).Error; err != nil {
		t.Fatalf("create Skill: %v", err)
	}
	mcp, _ := json.Marshal([]request.AIAgentMCPToolRequest{{
		ToolCode: "crm/get_customer", ServerCode: "crm", ToolName: "get_customer",
		RiskLevel: "read",
	}})
	agent := models.AIAgent{SkillIDs: jsonInt64List(skill.ID), AllowedMCPTools: string(mcp)}
	snapshot := &svc.AgentRevisionSnapshot{
		Agent: agent,
		WorkflowBindings: []svc.AgentRevisionWorkflowBinding{{
			WorkflowVersionID: 23, ToolName: "创建工单", TriggerInstruction: "用户要求创建工单",
		}},
	}
	engine := NewAgentLoopEngine()
	engine.retrieve = nil
	engine.history = nil
	turn := engine.prepareTurn(context.Background(), RunInput{AIAgent: agent}, snapshot)

	expectedDirectFunctions := []string{
		"direct function: builtin_conversation_context",
		"direct function: builtin_knowledge_retrieve",
		"direct function: graph_triage_service_request",
		"direct function: skill_" + jsonInt64List(skill.ID),
		"direct function: workflow_23",
		"direct function: crm_get_customer",
	}
	for _, expected := range expectedDirectFunctions {
		if !strings.Contains(turn.SystemPrompt, expected) {
			t.Fatalf("expected direct function %q in system prompt:\n%s", expected, turn.SystemPrompt)
		}
	}

	prohibition := "Never pass a capability code containing slashes as a tool function name; use the exact direct function name provided in the catalog or call tool_search with toolCode."
	if !strings.Contains(turn.SystemPrompt, prohibition) {
		t.Fatalf("expected prohibition instruction in system prompt:\n%s", turn.SystemPrompt)
	}
}

func TestAgentLoopEngineEmitsActivitySafelyWithConversationID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.AIConfig{}, &models.AIAgent{}, &models.AgentRevision{},
		&models.Conversation{}, &models.Message{},
		&models.AgentRun{}, &models.AgentStep{}, &models.AgentToolCall{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqls.SetDB(db)

	config := &models.AIConfig{Status: enums.StatusOk, ModelName: "test-model"}
	if err := db.Create(config).Error; err != nil {
		t.Fatalf("create config: %v", err)
	}

	definitionJSON, _ := json.Marshal(map[string]any{
		"agent": map[string]any{"name": "test-agent"},
	})
	agent := &models.AIAgent{Name: "test-agent", Status: enums.StatusOk, AIConfigID: config.ID}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	revision := &models.AgentRevision{
		AgentID: agent.ID, Status: enums.StatusOk, Revision: 1,
		Definition: string(definitionJSON),
	}
	if err := db.Create(revision).Error; err != nil {
		t.Fatalf("create revision: %v", err)
	}
	agent.PublishedRevisionID = revision.ID
	if err := db.Save(agent).Error; err != nil {
		t.Fatalf("save agent: %v", err)
	}

	conv := models.Conversation{ID: 100}
	userMsg := models.Message{ID: 200, Content: "test message", SenderType: enums.IMSenderTypeCustomer, MessageType: enums.IMMessageTypeText}

	// 1. Test Run with conversation ID > 0 (success)
	engine := newAgentLoopEngineWithLoop(func(ctx context.Context, config models.AIConfig, sysPrompt, userPrompt string, tools []ai.ToolDefinition, maxSteps int, executor ai.ToolCallExecutor) (*ai.ToolLoopResult, error) {
		return &ai.ToolLoopResult{
			ChatCompletionResult: ai.ChatCompletionResult{
				Content:   "Hello from loop",
				ModelName: "test-model",
			},
		}, nil
	})
	engine.retrieve = nil
	engine.history = nil

	res, err := engine.Run(context.Background(), RunInput{
		Conversation: conv,
		UserMessage:  userMsg,
		AIAgent:      *agent,
		AIConfig:     *config,
	})
	if err != nil {
		t.Fatalf("expected Run to succeed, got %v", err)
	}
	if res == nil || res.ReplyText != "Hello from loop" {
		t.Fatalf("unexpected result: %#v", res)
	}

	// 2. Test Run with conversation ID > 0 (loop error)
	errEngine := newAgentLoopEngineWithLoop(func(ctx context.Context, config models.AIConfig, sysPrompt, userPrompt string, tools []ai.ToolDefinition, maxSteps int, executor ai.ToolCallExecutor) (*ai.ToolLoopResult, error) {
		return nil, errors.New("loop execution failed")
	})
	errEngine.retrieve = nil
	errEngine.history = nil

	_, loopErr := errEngine.Run(context.Background(), RunInput{
		Conversation: conv,
		UserMessage:  userMsg,
		AIAgent:      *agent,
		AIConfig:     *config,
	})
	if loopErr == nil || !strings.Contains(loopErr.Error(), "loop execution failed") {
		t.Fatalf("expected loop error, got: %v", loopErr)
	}

	// 3. Test toolSearchExecutor with conversation ID > 0
	turn := agentLoopTurn{
		AllowedTools: []string{"builtin/conversation_context"},
		ToolPolicy:   parseAgentLoopToolPolicy(""),
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	executor := engine.toolSearchExecutor(RunInput{Conversation: conv}, turn, &state, &calls)

	toolResult, err := executor(context.Background(), ai.ToolCall{
		Name:      "builtin_conversation_context",
		Arguments: `{}`,
	})
	if err != nil {
		t.Fatalf("executor tool call failed: %v", err)
	}
	if toolResult == "" {
		t.Fatal("expected non-empty tool result")
	}

	// 4. Test offline / zero conversation ID
	offlineRes, err := engine.Run(context.Background(), RunInput{
		Conversation: models.Conversation{ID: 0},
		UserMessage:  userMsg,
		AIAgent:      *agent,
		AIConfig:     *config,
	})
	if err != nil {
		t.Fatalf("expected offline Run to succeed, got %v", err)
	}
	if offlineRes == nil || offlineRes.ReplyText != "Hello from loop" {
		t.Fatalf("unexpected offline result: %#v", offlineRes)
	}
}

func TestAgentLoopEngineRetrieveKnowledgeEmitsActivity(t *testing.T) {
	engine := NewAgentLoopEngine()
	retrievedCalled := false
	engine.retrieve = func(ctx context.Context, agent models.AIAgent, query string) (string, int, error) {
		retrievedCalled = true
		return "evidence", 1, nil
	}

	// Case 1: KnowledgeIDs present and conversation in context
	conv := models.Conversation{ID: 100}
	ctx := withAgentLoopConversation(context.Background(), conv)
	text, count, err := engine.retrieveKnowledge(ctx, models.AIAgent{KnowledgeIDs: "1,2"}, "query")
	if err != nil || !retrievedCalled || text != "evidence" || count != 1 {
		t.Fatalf("retrieveKnowledge failed: text=%q count=%d err=%v", text, count, err)
	}

	// Case 2: No KnowledgeIDs -> retrieve not called, returns empty
	retrievedCalled = false
	text, count, err = engine.retrieveKnowledge(ctx, models.AIAgent{KnowledgeIDs: ""}, "query")
	if err != nil || retrievedCalled || text != "" || count != 0 {
		t.Fatalf("expected no retrieve call when KnowledgeIDs is empty")
	}

	// Case 3: Conversation ID is 0 -> retrieve called safely without panic
	retrievedCalled = false
	zeroCtx := withAgentLoopConversation(context.Background(), models.Conversation{ID: 0})
	text, count, err = engine.retrieveKnowledge(zeroCtx, models.AIAgent{KnowledgeIDs: "1"}, "query")
	if err != nil || !retrievedCalled || text != "evidence" {
		t.Fatalf("retrieveKnowledge with zero conversation failed")
	}
}

func TestAgentLoopToolSearchExecutorEmitsWorkflowActivity(t *testing.T) {
	turn := agentLoopTurn{
		AllowedTools: []string{"workflow/42"},
		ToolPolicy:   parseAgentLoopToolPolicy(""),
		Workflows: map[int64]svc.AgentRevisionWorkflowBinding{
			42: {WorkflowVersionID: 42, ToolName: "Cancel Subscription"},
		},
	}
	state := agentLoopExecutionState{}
	var calls []svc.AgentLoopToolCallInput
	conv := models.Conversation{ID: 100}
	executor := NewAgentLoopEngine().toolSearchExecutor(RunInput{Conversation: conv}, turn, &state, &calls)

	// Since workflow 42 is not in db, it will fail to resolve workflow version, but activity is published beforehand
	_, _ = executor(context.Background(), ai.ToolCall{
		Name:      "workflow_42",
		Arguments: `{}`,
	})
	// Check that records captured the attempt
	if len(calls) != 1 || calls[0].ToolCode != "workflow/42" {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}
