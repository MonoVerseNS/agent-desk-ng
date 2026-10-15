package instruction

import (
	"encoding/json"
	"fmt"
	"strings"

	runtimetooling "agent-desk/internal/ai/runtime/tooling"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/toolx"
)

func BuildSelectedSkillActivationInstruction(skill *models.SkillDefinition) string {
	if skill == nil {
		return ""
	}
	lines := []string{
		"Active Specialized Skill:",
		fmt.Sprintf("- id: %d", skill.ID),
		fmt.Sprintf("- name: %s", strings.TrimSpace(skill.Name)),
	}
	if desc := strings.TrimSpace(skill.Description); desc != "" {
		lines = append(lines, fmt.Sprintf("- description: %s", desc))
	}
	lines = append(lines, "", "Execution Requirements:", "- Prioritize handling inquiries within this skill's scope this turn.", fmt.Sprintf("- When specialized handling details are needed, invoke %s to load skill documentation before proceeding.", toolx.BuiltinSkill.Name), "- If key information is missing, ask the customer for clarification first.", "- Do not invoke tools unauthorized for this skill.")
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func BuildSelectedSkillDocument(skill *models.SkillDefinition, toolDefinitions []runtimetooling.MCPToolDefinition) string {
	return BuildSkillDocument(skill, toolDefinitions)
}

func BuildSkillDocument(skill *models.SkillDefinition, toolDefinitions []runtimetooling.MCPToolDefinition) string {
	if skill == nil {
		return ""
	}
	lines := []string{
		"Active Specialized Skill:",
		fmt.Sprintf("- id: %d", skill.ID),
		fmt.Sprintf("- name: %s", strings.TrimSpace(skill.Name)),
	}
	if desc := strings.TrimSpace(skill.Description); desc != "" {
		lines = append(lines, fmt.Sprintf("- description: %s", desc))
	}
	if content := strings.TrimSpace(skill.Instruction); content != "" {
		lines = append(lines, "", "Skill Documentation:", content)
	}
	if examples := parseJSONStringArray(skill.Examples); len(examples) > 0 {
		lines = append(lines, "", "Typical Example Inquiries:")
		for _, item := range examples {
			lines = append(lines, "- "+item)
		}
	}
	if len(toolDefinitions) > 0 {
		lines = append(lines, "", "Authorized Tools for this Skill:")
		for _, item := range toolDefinitions {
			if strings.TrimSpace(item.ToolCode) == "" {
				continue
			}
			line := "- " + strings.TrimSpace(item.ToolCode)
			if title := strings.TrimSpace(item.Title); title != "" {
				line += " | " + title
			}
			lines = append(lines, line)
		}
	}
	lines = append(lines, "", "执行要求：", "- 优先遵循该技能说明完成任务。", "- 如果关键信息不足，先向用户追问。", "- 不得调用当前技能未授权的工具。")
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func parseJSONStringArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ret []string
	if err := json.Unmarshal([]byte(raw), &ret); err != nil {
		return nil
	}
	out := make([]string, 0, len(ret))
	for _, item := range ret {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}
