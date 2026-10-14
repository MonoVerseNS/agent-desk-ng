package tooling

import (
	"strings"
	"testing"

	"agent-desk/internal/ai/mcps"
)

func TestBuildReducedToolResultSummaryDeduplicatesStructuredAndTextContent(t *testing.T) {
	result := &mcps.ToolCallResult{
		StructuredContent: map[string]any{
			"timestamp": "2026-07-28 11:51:52",
			"timezone":  "Local",
		},
		Content: []mcps.ToolResultContent{{
			Type: "text",
			Text: `{"timezone":"Local","timestamp":"2026-07-28 11:51:52"}`,
		}},
	}

	summary := BuildReducedToolResultSummary(result)
	if strings.Count(summary, "timestamp") != 1 {
		t.Fatalf("duplicate MCP result was not removed: %q", summary)
	}
	if summary != `{"timestamp":"2026-07-28 11:51:52","timezone":"Local"}` {
		t.Fatalf("unexpected reduced result: %q", summary)
	}
}

func TestBuildReducedToolResultSummaryKeepsDistinctSegments(t *testing.T) {
	result := &mcps.ToolCallResult{
		StructuredContent: map[string]any{"status": "ok"},
		Content: []mcps.ToolResultContent{{
			Type: "text",
			Text: "additional context",
		}},
	}

	summary := BuildReducedToolResultSummary(result)
	if !strings.Contains(summary, `{"status":"ok"}`) || !strings.Contains(summary, "additional context") {
		t.Fatalf("distinct MCP result segments were lost: %q", summary)
	}
}

func TestBuildReducedToolResultSummaryTruncatesAt2500Runes(t *testing.T) {
	longText := strings.Repeat("x", 3000)
	result := &mcps.ToolCallResult{
		Content: []mcps.ToolResultContent{{
			Type: "text",
			Text: longText,
		}},
	}

	summary := BuildReducedToolResultSummary(result)
	expectedMarker := "[tool result reduced: original_length=3000, kept_length=2500]"
	if !strings.Contains(summary, expectedMarker) {
		t.Fatalf("expected reduction marker %q in summary, got: %q", expectedMarker, summary)
	}

	info := ParseReductionInfo(summary)
	if !info.Reduced || info.OriginalChars != 3000 || info.KeptChars != 2500 {
		t.Fatalf("unexpected ReductionInfo: %+v", info)
	}

	// Exactly 2500 runes should not be reduced
	exactText := strings.Repeat("y", 2500)
	exactResult := &mcps.ToolCallResult{
		Content: []mcps.ToolResultContent{{
			Type: "text",
			Text: exactText,
		}},
	}
	exactSummary := BuildReducedToolResultSummary(exactResult)
	if strings.Contains(exactSummary, "[tool result reduced:") {
		t.Fatalf("2500-rune payload should not be reduced, got: %q", exactSummary)
	}
	if exactSummary != exactText {
		t.Fatalf("expected exact text, got length %d", len(exactSummary))
	}

	// Multibyte runes test
	chineseText := strings.Repeat("测", 2600)
	chineseResult := &mcps.ToolCallResult{
		Content: []mcps.ToolResultContent{{
			Type: "text",
			Text: chineseText,
		}},
	}
	chineseSummary := BuildReducedToolResultSummary(chineseResult)
	expectedChineseMarker := "[tool result reduced: original_length=2600, kept_length=2500]"
	if !strings.Contains(chineseSummary, expectedChineseMarker) {
		t.Fatalf("expected multibyte reduction marker %q, got: %q", expectedChineseMarker, chineseSummary)
	}
	chineseInfo := ParseReductionInfo(chineseSummary)
	if !chineseInfo.Reduced || chineseInfo.OriginalChars != 2600 || chineseInfo.KeptChars != 2500 {
		t.Fatalf("unexpected chinese ReductionInfo: %+v", chineseInfo)
	}
}
