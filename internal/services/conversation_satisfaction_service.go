package services

import (
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/repositories"
	"strings"

	"github.com/mlogclub/simple/sqls"
)

var ConversationSatisfactionService = newConversationSatisfactionService()

func newConversationSatisfactionService() *conversationSatisfactionService {
	return &conversationSatisfactionService{}
}

type conversationSatisfactionService struct{}

func (s *conversationSatisfactionService) SubmitSatisfaction(conversationID int64, externalUser openidentity.ExternalUser, score int, feedback string) (*models.ConversationSatisfaction, error) {
	if score < 1 || score > 5 {
		return nil, errorsx.InvalidParamI18n("error.invalidParam")
	}

	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return nil, errorsx.InvalidParamI18n("error.e0116")
	}
	if !ConversationService.IsCustomerConversationOwner(conversation, externalUser) {
		return nil, errorsx.ForbiddenI18n("error.e0222")
	}

	trimmedFeedback := strings.TrimSpace(feedback)
	operatorName := strings.TrimSpace(externalUser.ExternalName)
	if operatorName == "" {
		operatorName = strings.TrimSpace(externalUser.ExternalID)
	}

	var result *models.ConversationSatisfaction

	err := sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		existing := repositories.ConversationSatisfactionRepository.GetByConversationID(ctx.Tx, conversationID)
		if existing != nil {
			existing.Score = score
			existing.Feedback = trimmedFeedback
			existing.UpdateUserName = operatorName
			if err := repositories.ConversationSatisfactionRepository.Update(ctx.Tx, existing); err != nil {
				return err
			}
			result = existing
			return nil
		}

		item := &models.ConversationSatisfaction{
			ConversationID: conversation.ID,
			Score:          score,
			Feedback:       trimmedFeedback,
			ChannelID:      conversation.ChannelID,
			CustomerID:     conversation.CustomerID,
			AssigneeID:     conversation.CurrentAssigneeID,
			AIAgentID:      conversation.AIAgentID,
			AuditFields: models.AuditFields{
				CreateUserName: operatorName,
				UpdateUserName: operatorName,
			},
		}
		if err := repositories.ConversationSatisfactionRepository.Create(ctx.Tx, item); err != nil {
			return err
		}
		result = item
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *conversationSatisfactionService) GetByConversationID(conversationID int64) *models.ConversationSatisfaction {
	if conversationID <= 0 {
		return nil
	}
	return repositories.ConversationSatisfactionRepository.GetByConversationID(sqls.DB(), conversationID)
}
