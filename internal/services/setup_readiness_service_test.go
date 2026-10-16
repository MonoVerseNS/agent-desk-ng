package services_test

import (
	"strings"
	"testing"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/services"

	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func setupSetupReadinessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.CustomerIdentity{},
		&models.AIAgent{},
		&models.AgentTeam{},
		&models.AgentTeamSchedule{},
		&models.AgentProfile{},
		&models.Channel{},
		&models.Conversation{},
		&models.ConversationParticipant{},
		&models.ConversationAssignment{},
		&models.ConversationEventLog{},
		&models.ConversationReadState{},
		&models.Message{},
		&models.ChannelMessageOutbox{},
	); err != nil {
		t.Fatalf("auto migrate error = %v", err)
	}
	sqls.SetDB(db)
	return db
}

func TestSetupReadiness_EmptyDBAndQuickSetupAll(t *testing.T) {
	db := setupSetupReadinessTestDB(t)

	admin := models.User{
		Username: "admin",
		Nickname: "Admin Operator",
		Status:   enums.StatusOk,
	}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin error = %v", err)
	}
	principal := &dto.AuthPrincipal{
		UserID:   admin.ID,
		Username: admin.Username,
		Nickname: admin.Nickname,
	}

	readiness, err := services.SetupReadinessService.GetReadiness(principal)
	if err != nil {
		t.Fatalf("GetReadiness error = %v", err)
	}
	if readiness.CanReceiveRequestsNow {
		t.Fatalf("expected CanReceiveRequestsNow = false on empty DB")
	}
	if readiness.ReadySteps != 0 {
		t.Fatalf("expected ReadySteps = 0 on empty DB, got %d", readiness.ReadySteps)
	}
	if readiness.TotalSteps != 6 {
		t.Fatalf("expected TotalSteps = 6, got %d", readiness.TotalSteps)
	}
	if readiness.CurrentOperator.CanReceiveNow {
		t.Fatalf("expected CurrentOperator.CanReceiveNow = false on empty DB")
	}

	fixResp, err := services.SetupReadinessService.ApplyQuickFix(request.SetupQuickFixRequest{
		Action: "quick_setup_all",
	}, principal)
	if err != nil {
		t.Fatalf("ApplyQuickFix(quick_setup_all) error = %v", err)
	}
	if !fixResp.Readiness.CanReceiveRequestsNow {
		t.Fatalf("expected CanReceiveRequestsNow = true after quick_setup_all, steps = %+v", fixResp.Readiness.Steps)
	}
	if fixResp.Readiness.ReadySteps != 6 {
		t.Fatalf("expected ReadySteps = 6 after quick_setup_all, got %d", fixResp.Readiness.ReadySteps)
	}
	if !fixResp.Readiness.CurrentOperator.CanReceiveNow {
		t.Fatalf("expected CurrentOperator.CanReceiveNow = true after quick_setup_all, blockers = %v", fixResp.Readiness.CurrentOperator.BlockerCodes)
	}

	// Second call must be idempotent
	fixResp2, err := services.SetupReadinessService.ApplyQuickFix(request.SetupQuickFixRequest{
		Action: "quick_setup_all",
	}, principal)
	if err != nil {
		t.Fatalf("second ApplyQuickFix(quick_setup_all) error = %v", err)
	}
	if !fixResp2.Readiness.CanReceiveRequestsNow {
		t.Fatalf("expected CanReceiveRequestsNow = true after second quick_setup_all")
	}

	var teamCount int64
	db.Model(&models.AgentTeam{}).Count(&teamCount)
	if teamCount != 1 {
		t.Fatalf("expected idempotent 1 team, got %d", teamCount)
	}
}

func TestSetupReadiness_OffHoursAndBusyOperator(t *testing.T) {
	db := setupSetupReadinessTestDB(t)

	now := time.Now()
	opUser := models.User{
		Username: "operator1",
		Nickname: "Operator One",
		Status:   enums.StatusOk,
	}
	if err := db.Create(&opUser).Error; err != nil {
		t.Fatalf("create user error = %v", err)
	}
	team := models.AgentTeam{
		Name:   "Support Team",
		Status: enums.StatusOk,
	}
	if err := db.Create(&team).Error; err != nil {
		t.Fatalf("create team error = %v", err)
	}
	// Schedule in the past (off-hours now)
	pastSchedule := models.AgentTeamSchedule{
		TeamID:  team.ID,
		StartAt: now.Add(-48 * time.Hour),
		EndAt:   now.Add(-24 * time.Hour),
		Status:  enums.StatusOk,
	}
	if err := db.Create(&pastSchedule).Error; err != nil {
		t.Fatalf("create past schedule error = %v", err)
	}
	// Operator is Busy and AutoAssignEnabled = false
	profile := models.AgentProfile{
		UserID:             opUser.ID,
		TeamID:             team.ID,
		AgentCode:          "OP-001",
		DisplayName:        "Operator One",
		ServiceStatus:      enums.ServiceStatusBusy,
		MaxConcurrentCount: 5,
		AutoAssignEnabled:  false,
		Status:             enums.StatusOk,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile error = %v", err)
	}
	if err := db.Model(&profile).Update("auto_assign_enabled", false).Error; err != nil {
		t.Fatalf("update auto_assign_enabled error = %v", err)
	}

	principal := &dto.AuthPrincipal{
		UserID:   opUser.ID,
		Username: opUser.Username,
		Nickname: opUser.Nickname,
	}

	readiness, err := services.SetupReadinessService.GetReadiness(principal)
	if err != nil {
		t.Fatalf("GetReadiness error = %v", err)
	}
	if readiness.CurrentOperator.TeamHasScheduleNow {
		t.Fatalf("expected TeamHasScheduleNow = false for past schedule")
	}
	if readiness.CurrentOperator.CanReceiveNow {
		t.Fatalf("expected CurrentOperator.CanReceiveNow = false when busy and off-hours")
	}

	// Fix operator profile and schedule
	_, err = services.SetupReadinessService.ApplyQuickFix(request.SetupQuickFixRequest{
		Action: "enable_my_operator_profile",
	}, principal)
	if err != nil {
		t.Fatalf("enable_my_operator_profile error = %v", err)
	}
	readinessAfter, err := services.SetupReadinessService.GetReadiness(principal)
	if err != nil {
		t.Fatalf("GetReadiness after fix error = %v", err)
	}
	if !readinessAfter.CurrentOperator.CanReceiveNow {
		t.Fatalf("expected CurrentOperator.CanReceiveNow = true after enable_my_operator_profile, blockers = %v", readinessAfter.CurrentOperator.BlockerCodes)
	}
}

func TestSetupReadiness_SimulateRoute_AIOnlyAndMissingTeams(t *testing.T) {
	db := setupSetupReadinessTestDB(t)

	opUser := models.User{
		Username: "op2",
		Nickname: "Operator Two",
		Status:   enums.StatusOk,
	}
	db.Create(&opUser)
	principal := &dto.AuthPrincipal{
		UserID:   opUser.ID,
		Username: opUser.Username,
		Nickname: opUser.Nickname,
	}

	// Setup team + active schedule + idle operator
	_, err := services.SetupReadinessService.ApplyQuickFix(request.SetupQuickFixRequest{
		Action: "enable_my_operator_profile",
	}, principal)
	if err != nil {
		t.Fatalf("enable_my_operator_profile error = %v", err)
	}

	// Create AI Agent in AIOnly mode and empty TeamIDs
	agent := models.AIAgent{
		Name:           "AI Bot",
		Status:         enums.StatusOk,
		ServiceMode:    enums.IMConversationServiceModeAIOnly,
		RolloutPercent: 100,
		TeamIDs:        "",
	}
	db.Create(&agent)

	channel := models.Channel{
		Name:                  "Web Widget",
		ChannelType:           "web",
		ChannelID:             "web-test-1",
		AIAgentID:             agent.ID,
		AIAgentRolloutPercent: 100,
		Status:                enums.StatusOk,
	}
	db.Create(&channel)

	sim1, err := services.SetupReadinessService.SimulateRoute(request.SetupSimulateRouteRequest{
		ChannelID: channel.ID,
		BypassAI:  true,
	}, principal)
	if err != nil {
		t.Fatalf("SimulateRoute 1 error = %v", err)
	}
	if sim1.WillReachOperator {
		t.Fatalf("expected WillReachOperator = false when AIOnly")
	}
	if sim1.FinalOutcomeCode != "blocked_ai_only_mode" {
		t.Fatalf("expected blocked_ai_only_mode, got %s", sim1.FinalOutcomeCode)
	}

	// Switch to HumanOnly but still empty TeamIDs
	db.Model(&agent).Update("service_mode", enums.IMConversationServiceModeHumanOnly)
	sim2, err := services.SetupReadinessService.SimulateRoute(request.SetupSimulateRouteRequest{
		ChannelID: channel.ID,
		BypassAI:  true,
	}, principal)
	if err != nil {
		t.Fatalf("SimulateRoute 2 error = %v", err)
	}
	if sim2.WillReachOperator {
		t.Fatalf("expected WillReachOperator = false when TeamIDs is empty")
	}
	if sim2.FinalOutcomeCode != "blocked_agent_no_teams" {
		t.Fatalf("expected blocked_agent_no_teams, got %s", sim2.FinalOutcomeCode)
	}

	// Fix AI Agent handoff via quick fix
	_, err = services.SetupReadinessService.ApplyQuickFix(request.SetupQuickFixRequest{
		Action:   "fix_ai_agent_handoff",
		EntityID: agent.ID,
	}, principal)
	if err != nil {
		t.Fatalf("fix_ai_agent_handoff error = %v", err)
	}

	sim3, err := services.SetupReadinessService.SimulateRoute(request.SetupSimulateRouteRequest{
		ChannelID: channel.ID,
		BypassAI:  true,
	}, principal)
	if err != nil {
		t.Fatalf("SimulateRoute 3 error = %v", err)
	}
	if !sim3.WillReachOperator {
		t.Fatalf("expected WillReachOperator = true after fix_ai_agent_handoff, got outcome %s", sim3.FinalOutcomeCode)
	}
	if sim3.FinalOutcomeCode != "assigned_to_operator" {
		t.Fatalf("expected assigned_to_operator, got %s", sim3.FinalOutcomeCode)
	}
	if sim3.MatchedOperator == nil || sim3.MatchedOperator.UserID != opUser.ID {
		t.Fatalf("expected MatchedOperator UserID = %d, got %+v", opUser.ID, sim3.MatchedOperator)
	}
}
