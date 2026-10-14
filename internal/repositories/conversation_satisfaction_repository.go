package repositories

import (
	"agent-desk/internal/models"

	"gorm.io/gorm"
)

var ConversationSatisfactionRepository = newConversationSatisfactionRepository()

func newConversationSatisfactionRepository() *conversationSatisfactionRepository {
	return &conversationSatisfactionRepository{}
}

type conversationSatisfactionRepository struct{}

func (r *conversationSatisfactionRepository) Get(db *gorm.DB, id int64) *models.ConversationSatisfaction {
	if id <= 0 {
		return nil
	}
	item := &models.ConversationSatisfaction{}
	if err := db.First(item, "id = ?", id).Error; err != nil {
		return nil
	}
	return item
}

func (r *conversationSatisfactionRepository) GetByConversationID(db *gorm.DB, conversationID int64) *models.ConversationSatisfaction {
	if conversationID <= 0 {
		return nil
	}
	item := &models.ConversationSatisfaction{}
	if err := db.Where("conversation_id = ?", conversationID).First(item).Error; err != nil {
		return nil
	}
	return item
}

func (r *conversationSatisfactionRepository) Create(db *gorm.DB, item *models.ConversationSatisfaction) error {
	return db.Create(item).Error
}

func (r *conversationSatisfactionRepository) Update(db *gorm.DB, item *models.ConversationSatisfaction) error {
	return db.Save(item).Error
}

func (r *conversationSatisfactionRepository) Count(db *gorm.DB, where ...any) int64 {
	var count int64
	tx := db.Model(&models.ConversationSatisfaction{})
	if len(where) > 0 {
		tx = tx.Where(where[0], where[1:]...)
	}
	tx.Count(&count)
	return count
}

func (r *conversationSatisfactionRepository) AverageScore(db *gorm.DB, where ...any) float64 {
	var avg *float64
	tx := db.Model(&models.ConversationSatisfaction{})
	if len(where) > 0 {
		tx = tx.Where(where[0], where[1:]...)
	}
	if err := tx.Select("AVG(score)").Scan(&avg).Error; err != nil || avg == nil {
		return 0
	}
	return *avg
}
