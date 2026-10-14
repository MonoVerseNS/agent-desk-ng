package runtime

import (
	"context"
	"strings"

	applicationruntime "agent-desk/internal/ai/application/runtime"
	svc "agent-desk/internal/services"
)

var AIReplyService = newAIReplyService()

func init() {
	svc.TriggerAIReplyAsyncHook = AIReplyService.TriggerReplyAsync
}

type replyExecutor interface {
	Run(ctx context.Context, input runtimeReplyRunInput) (*applicationruntime.RunResult, error)
	ResumePendingInterrupt(ctx context.Context, input runtimeReplyResumeInput) (*applicationruntime.RunResult, error)
}

func newAIReplyService() *aiReplyService {
	return newAIReplyServiceWithExecutor(newRuntimeReplyExecutor())
}

func newAIReplyServiceWithExecutor(executor replyExecutor) *aiReplyService {
	return &aiReplyService{
		eligibility: newReplyEligibility(),
		executor:    executor,
		interrupts:  newReplyInterruptService(),
		commit:      newReplyCommitService(),
	}
}

type aiReplyService struct {
	eligibility *replyEligibility
	executor    replyExecutor
	interrupts  *replyInterruptService
	commit      *replyCommitService
}

func firstInvokedToolCode(summary *applicationruntime.RunResult) string {
	if summary == nil {
		return ""
	}
	if len(summary.InvokedToolCodes) > 0 {
		return strings.TrimSpace(summary.InvokedToolCodes[0])
	}
	return ""
}
