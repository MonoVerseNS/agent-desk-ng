package services

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"agent-desk/internal/models"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/utils"

	"github.com/mlogclub/simple/sqls"
)

var SetupReadinessService = newSetupReadinessService()

func newSetupReadinessService() *setupReadinessService {
	return &setupReadinessService{}
}

type setupReadinessService struct{}

const (
	QuickFixActionQuickSetupAll        = "quick_setup_all"
	QuickFixActionEnsureTeamSchedule   = "ensure_team_and_schedule"
	QuickFixActionEnableMyProfile      = "enable_my_operator_profile"
	QuickFixActionFixAIAgentHandoff    = "fix_ai_agent_handoff"
	QuickFixActionEnsureDefaultChannel = "ensure_default_channel"
)

func (s *setupReadinessService) GetReadiness(operator *dto.AuthPrincipal) (*response.SetupReadinessResponse, error) {
	now := time.Now()
	db := sqls.DB()

	var teams []models.AgentTeam
	if err := db.Order("id ASC").Find(&teams).Error; err != nil {
		return nil, err
	}
	activeTeamMap := make(map[int64]models.AgentTeam)
	activeTeamIDs := make([]int64, 0, len(teams))
	for _, team := range teams {
		if team.Status == enums.StatusOk {
			activeTeamMap[team.ID] = team
			activeTeamIDs = append(activeTeamIDs, team.ID)
		}
	}

	var schedules []models.AgentTeamSchedule
	if err := db.Order("id ASC").Find(&schedules).Error; err != nil {
		return nil, err
	}
	activeScheduleNowTeamSet := make(map[int64]bool)
	anyScheduleTeamSet := make(map[int64]bool)
	for _, sch := range schedules {
		if sch.Status != enums.StatusOk {
			continue
		}
		anyScheduleTeamSet[sch.TeamID] = true
		if !sch.StartAt.After(now) && sch.EndAt.After(now) {
			activeScheduleNowTeamSet[sch.TeamID] = true
		}
	}

	var profiles []models.AgentProfile
	if err := db.Order("id ASC").Find(&profiles).Error; err != nil {
		return nil, err
	}

	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}
	activeUserMap := make(map[int64]models.User, len(users))
	for _, u := range users {
		if u.Status == enums.StatusOk && u.DeletedAt == nil {
			activeUserMap[u.ID] = u
		}
	}

	activeCounts, err := ConversationDispatchService.findActiveConversationCountMap(extractProfileUserIDs(profiles))
	if err != nil {
		return nil, err
	}

	var aiAgents []models.AIAgent
	if err := db.Order("id ASC").Find(&aiAgents).Error; err != nil {
		return nil, err
	}

	var channels []models.Channel
	if err := db.Order("id ASC").Find(&channels).Error; err != nil {
		return nil, err
	}

	// Step 1: Operator Teams
	step1 := s.evaluateTeamsStep(teams, activeTeamIDs)

	// Step 2: Shift Schedules (Active Now)
	step2 := s.evaluateSchedulesStep(teams, activeTeamMap, activeScheduleNowTeamSet, anyScheduleTeamSet)

	// Step 3: Operator Profiles
	step3, readyOperatorTeams := s.evaluateOperatorsStep(profiles, activeTeamMap, activeScheduleNowTeamSet, activeUserMap, activeCounts)

	// Step 4: AI Agents & Handoff Configuration
	step4, readyAIAgentMap := s.evaluateAIAgentsStep(aiAgents, activeTeamMap, activeScheduleNowTeamSet, readyOperatorTeams)

	// Step 5: Inbound Channels
	step5, channelBriefs := s.evaluateChannelsStep(channels, aiAgents)

	// Step 6: End-to-End Routing Path
	step6, canReceiveRequestsNow := s.evaluateEndToEndStep(channels, aiAgents, readyAIAgentMap, activeTeamMap, activeScheduleNowTeamSet, readyOperatorTeams)

	steps := []response.SetupStepItem{step1, step2, step3, step4, step5, step6}
	readySteps := 0
	hasError := false
	hasWarning := false
	for _, st := range steps {
		switch st.Status {
		case "ok":
			readySteps++
		case "error":
			hasError = true
		case "warning":
			hasWarning = true
		}
	}

	overallStatus := "ok"
	if hasError || !canReceiveRequestsNow {
		if hasError {
			overallStatus = "error"
		} else {
			overallStatus = "warning"
		}
	} else if hasWarning {
		overallStatus = "warning"
	}

	currentOp := s.evaluateCurrentOperator(operator, profiles, activeTeamMap, activeScheduleNowTeamSet, activeCounts)

	return &response.SetupReadinessResponse{
		OverallStatus:         overallStatus,
		ReadySteps:            readySteps,
		TotalSteps:            len(steps),
		CanReceiveRequestsNow: canReceiveRequestsNow,
		Steps:                 steps,
		CurrentOperator:       currentOp,
		Channels:              channelBriefs,
	}, nil
}

func (s *setupReadinessService) evaluateTeamsStep(teams []models.AgentTeam, activeTeamIDs []int64) response.SetupStepItem {
	issues := make([]response.SetupIssueItem, 0)
	quickFixes := make([]string, 0)
	status := "ok"
	summaryCode := "ok"

	if len(activeTeamIDs) == 0 {
		status = "error"
		summaryCode = "no_active_team"
		issues = append(issues, response.SetupIssueItem{
			Code:           "no_active_team",
			Severity:       "error",
			EntityType:     "team",
			QuickFixAction: QuickFixActionEnsureTeamSchedule,
		})
		quickFixes = append(quickFixes, QuickFixActionEnsureTeamSchedule)
	}

	return response.SetupStepItem{
		Key:             "teams",
		StepNumber:      1,
		Status:          status,
		SummaryCode:     summaryCode,
		TotalCount:      len(teams),
		ReadyCount:      len(activeTeamIDs),
		Issues:          issues,
		QuickFixActions: quickFixes,
		ManagePath:      "/dashboard/agents",
	}
}

func (s *setupReadinessService) evaluateSchedulesStep(
	teams []models.AgentTeam,
	activeTeamMap map[int64]models.AgentTeam,
	activeScheduleNowTeamSet map[int64]bool,
	anyScheduleTeamSet map[int64]bool,
) response.SetupStepItem {
	issues := make([]response.SetupIssueItem, 0)
	quickFixes := make([]string, 0)
	readyTeamCount := 0

	for _, team := range teams {
		if team.Status != enums.StatusOk {
			continue
		}
		if activeScheduleNowTeamSet[team.ID] {
			readyTeamCount++
			continue
		}
		issueCode := "team_no_schedule_configured"
		if anyScheduleTeamSet[team.ID] {
			issueCode = "team_off_hours_now"
		}
		issues = append(issues, response.SetupIssueItem{
			Code:           issueCode,
			Severity:       "warning",
			EntityType:     "team",
			EntityID:       team.ID,
			EntityName:     team.Name,
			QuickFixAction: QuickFixActionEnsureTeamSchedule,
		})
	}

	status := "ok"
	summaryCode := "ok"
	if len(activeTeamMap) == 0 {
		status = "error"
		summaryCode = "no_active_team"
		quickFixes = append(quickFixes, QuickFixActionEnsureTeamSchedule)
	} else if readyTeamCount == 0 {
		status = "error"
		summaryCode = "no_schedule_now"
		quickFixes = append(quickFixes, QuickFixActionEnsureTeamSchedule)
	} else if len(issues) > 0 {
		status = "warning"
		summaryCode = "partial_schedule_now"
		quickFixes = append(quickFixes, QuickFixActionEnsureTeamSchedule)
	}

	return response.SetupStepItem{
		Key:             "schedules",
		StepNumber:      2,
		Status:          status,
		SummaryCode:     summaryCode,
		TotalCount:      len(activeTeamMap),
		ReadyCount:      readyTeamCount,
		Issues:          issues,
		QuickFixActions: quickFixes,
		ManagePath:      "/dashboard/agent-team-schedules",
	}
}

func (s *setupReadinessService) evaluateOperatorsStep(
	profiles []models.AgentProfile,
	activeTeamMap map[int64]models.AgentTeam,
	activeScheduleNowTeamSet map[int64]bool,
	activeUserMap map[int64]models.User,
	activeCounts map[int64]int,
) (response.SetupStepItem, map[int64]bool) {
	issues := make([]response.SetupIssueItem, 0)
	quickFixes := make([]string, 0)
	readyCount := 0
	readyOperatorTeams := make(map[int64]bool)

	for _, p := range profiles {
		displayName := strings.TrimSpace(p.DisplayName)
		if displayName == "" {
			if u, ok := activeUserMap[p.UserID]; ok {
				displayName = firstNonEmpty(u.Nickname, u.Username)
			}
		}
		if displayName == "" {
			displayName = fmt.Sprintf("#%d", p.UserID)
		}

		if p.Status != enums.StatusOk {
			issues = append(issues, response.SetupIssueItem{
				Code:           "operator_profile_disabled",
				Severity:       "warning",
				EntityType:     "operator",
				EntityID:       p.ID,
				EntityName:     displayName,
				QuickFixAction: QuickFixActionEnableMyProfile,
			})
			continue
		}
		if _, userOk := activeUserMap[p.UserID]; !userOk {
			issues = append(issues, response.SetupIssueItem{
				Code:       "operator_user_disabled",
				Severity:   "warning",
				EntityType: "operator",
				EntityID:   p.ID,
				EntityName: displayName,
			})
			continue
		}
		if _, teamOk := activeTeamMap[p.TeamID]; !teamOk {
			issues = append(issues, response.SetupIssueItem{
				Code:           "operator_no_active_team",
				Severity:       "error",
				EntityType:     "operator",
				EntityID:       p.ID,
				EntityName:     displayName,
				QuickFixAction: QuickFixActionEnableMyProfile,
			})
			continue
		}
		if !p.AutoAssignEnabled {
			issues = append(issues, response.SetupIssueItem{
				Code:           "operator_auto_assign_off",
				Severity:       "warning",
				EntityType:     "operator",
				EntityID:       p.ID,
				EntityName:     displayName,
				QuickFixAction: QuickFixActionEnableMyProfile,
			})
			continue
		}
		if p.ServiceStatus != enums.ServiceStatusIdle {
			issues = append(issues, response.SetupIssueItem{
				Code:           "operator_busy",
				Severity:       "warning",
				EntityType:     "operator",
				EntityID:       p.ID,
				EntityName:     displayName,
				QuickFixAction: QuickFixActionEnableMyProfile,
			})
			continue
		}
		activeCount := activeCounts[p.UserID]
		if p.MaxConcurrentCount <= 0 || activeCount >= p.MaxConcurrentCount {
			issues = append(issues, response.SetupIssueItem{
				Code:           "operator_at_capacity",
				Severity:       "warning",
				EntityType:     "operator",
				EntityID:       p.ID,
				EntityName:     displayName,
				QuickFixAction: QuickFixActionEnableMyProfile,
			})
			continue
		}

		readyCount++
		if activeScheduleNowTeamSet[p.TeamID] {
			readyOperatorTeams[p.TeamID] = true
		}
	}

	status := "ok"
	summaryCode := "ok"
	if len(profiles) == 0 {
		status = "error"
		summaryCode = "no_operator_profile"
		issues = append(issues, response.SetupIssueItem{
			Code:           "no_operator_profile",
			Severity:       "error",
			EntityType:     "operator",
			QuickFixAction: QuickFixActionEnableMyProfile,
		})
		quickFixes = append(quickFixes, QuickFixActionEnableMyProfile)
	} else if readyCount == 0 {
		status = "error"
		summaryCode = "no_idle_operator"
		quickFixes = append(quickFixes, QuickFixActionEnableMyProfile)
	} else if len(issues) > 0 {
		status = "warning"
		summaryCode = "some_operators_unavailable"
	}

	return response.SetupStepItem{
		Key:             "operators",
		StepNumber:      3,
		Status:          status,
		SummaryCode:     summaryCode,
		TotalCount:      len(profiles),
		ReadyCount:      readyCount,
		Issues:          issues,
		QuickFixActions: quickFixes,
		ManagePath:      "/dashboard/agents",
	}, readyOperatorTeams
}

func (s *setupReadinessService) evaluateAIAgentsStep(
	aiAgents []models.AIAgent,
	activeTeamMap map[int64]models.AgentTeam,
	activeScheduleNowTeamSet map[int64]bool,
	readyOperatorTeams map[int64]bool,
) (response.SetupStepItem, map[int64]bool) {
	issues := make([]response.SetupIssueItem, 0)
	quickFixes := make([]string, 0)
	readyCount := 0
	readyAIAgentMap := make(map[int64]bool)

	activeAgentCount := 0
	for _, agent := range aiAgents {
		if agent.Status != enums.StatusOk {
			continue
		}
		activeAgentCount++
		if agent.ServiceMode == enums.IMConversationServiceModeAIOnly {
			issues = append(issues, response.SetupIssueItem{
				Code:           "agent_ai_only_mode",
				Severity:       "warning",
				EntityType:     "ai_agent",
				EntityID:       agent.ID,
				EntityName:     agent.Name,
				QuickFixAction: QuickFixActionFixAIAgentHandoff,
			})
			continue
		}
		teamIDs := utils.SplitInt64s(agent.TeamIDs)
		validTeamIDs := make([]int64, 0, len(teamIDs))
		for _, tid := range teamIDs {
			if _, ok := activeTeamMap[tid]; ok {
				validTeamIDs = append(validTeamIDs, tid)
			}
		}
		if len(validTeamIDs) == 0 {
			issues = append(issues, response.SetupIssueItem{
				Code:           "agent_no_teams",
				Severity:       "error",
				EntityType:     "ai_agent",
				EntityID:       agent.ID,
				EntityName:     agent.Name,
				QuickFixAction: QuickFixActionFixAIAgentHandoff,
			})
			continue
		}
		if agent.ServiceMode == enums.IMConversationServiceModeAIFirst && !agent.AutoHandoffOnFailure {
			issues = append(issues, response.SetupIssueItem{
				Code:           "agent_auto_handoff_disabled",
				Severity:       "warning",
				EntityType:     "ai_agent",
				EntityID:       agent.ID,
				EntityName:     agent.Name,
				QuickFixAction: QuickFixActionFixAIAgentHandoff,
			})
		}

		readyCount++
		for _, tid := range validTeamIDs {
			if activeScheduleNowTeamSet[tid] && readyOperatorTeams[tid] {
				readyAIAgentMap[agent.ID] = true
				break
			}
		}
	}

	status := "ok"
	summaryCode := "ok"
	if activeAgentCount == 0 {
		status = "error"
		summaryCode = "no_active_ai_agent"
		issues = append(issues, response.SetupIssueItem{
			Code:           "no_active_ai_agent",
			Severity:       "error",
			EntityType:     "ai_agent",
			QuickFixAction: QuickFixActionFixAIAgentHandoff,
		})
		quickFixes = append(quickFixes, QuickFixActionFixAIAgentHandoff)
	} else if readyCount == 0 {
		status = "error"
		summaryCode = "no_handoff_ready_agent"
		quickFixes = append(quickFixes, QuickFixActionFixAIAgentHandoff)
	} else if len(issues) > 0 {
		status = "warning"
		summaryCode = "agent_has_warnings"
		quickFixes = append(quickFixes, QuickFixActionFixAIAgentHandoff)
	}

	return response.SetupStepItem{
		Key:             "ai_agents",
		StepNumber:      4,
		Status:          status,
		SummaryCode:     summaryCode,
		TotalCount:      len(aiAgents),
		ReadyCount:      readyCount,
		Issues:          issues,
		QuickFixActions: quickFixes,
		ManagePath:      "/dashboard/ai-agents",
	}, readyAIAgentMap
}

func (s *setupReadinessService) evaluateChannelsStep(
	channels []models.Channel,
	aiAgents []models.AIAgent,
) (response.SetupStepItem, []response.SetupChannelBrief) {
	issues := make([]response.SetupIssueItem, 0)
	quickFixes := make([]string, 0)
	readyCount := 0

	agentMap := make(map[int64]models.AIAgent, len(aiAgents))
	for _, a := range aiAgents {
		if a.Status == enums.StatusOk {
			agentMap[a.ID] = a
		}
	}

	briefs := make([]response.SetupChannelBrief, 0, len(channels))
	activeChannelCount := 0
	for _, ch := range channels {
		briefs = append(briefs, response.SetupChannelBrief{
			ID:             ch.ID,
			Name:           ch.Name,
			ChannelType:    ch.ChannelType,
			ChannelID:      ch.ChannelID,
			AIAgentID:      ch.AIAgentID,
			RolloutPercent: ch.AIAgentRolloutPercent,
			Status:         int(ch.Status),
		})
		if ch.Status != enums.StatusOk {
			continue
		}
		activeChannelCount++
		if ch.AIAgentID <= 0 {
			issues = append(issues, response.SetupIssueItem{
				Code:           "channel_no_agent",
				Severity:       "error",
				EntityType:     "channel",
				EntityID:       ch.ID,
				EntityName:     ch.Name,
				QuickFixAction: QuickFixActionEnsureDefaultChannel,
			})
			continue
		}
		if _, ok := agentMap[ch.AIAgentID]; !ok {
			issues = append(issues, response.SetupIssueItem{
				Code:           "channel_agent_disabled",
				Severity:       "error",
				EntityType:     "channel",
				EntityID:       ch.ID,
				EntityName:     ch.Name,
				QuickFixAction: QuickFixActionEnsureDefaultChannel,
			})
			continue
		}
		if ch.AIAgentRolloutPercent <= 0 {
			issues = append(issues, response.SetupIssueItem{
				Code:           "channel_zero_rollout",
				Severity:       "error",
				EntityType:     "channel",
				EntityID:       ch.ID,
				EntityName:     ch.Name,
				QuickFixAction: QuickFixActionEnsureDefaultChannel,
			})
			continue
		}
		readyCount++
	}

	status := "ok"
	summaryCode := "ok"
	if activeChannelCount == 0 {
		status = "error"
		summaryCode = "no_active_channel"
		issues = append(issues, response.SetupIssueItem{
			Code:           "no_active_channel",
			Severity:       "error",
			EntityType:     "channel",
			QuickFixAction: QuickFixActionEnsureDefaultChannel,
		})
		quickFixes = append(quickFixes, QuickFixActionEnsureDefaultChannel)
	} else if readyCount == 0 {
		status = "error"
		summaryCode = "no_ready_channel"
		quickFixes = append(quickFixes, QuickFixActionEnsureDefaultChannel)
	} else if len(issues) > 0 {
		status = "warning"
		summaryCode = "some_channels_misconfigured"
		quickFixes = append(quickFixes, QuickFixActionEnsureDefaultChannel)
	}

	return response.SetupStepItem{
		Key:             "channels",
		StepNumber:      5,
		Status:          status,
		SummaryCode:     summaryCode,
		TotalCount:      len(channels),
		ReadyCount:      readyCount,
		Issues:          issues,
		QuickFixActions: quickFixes,
		ManagePath:      "/dashboard/channels",
	}, briefs
}

func (s *setupReadinessService) evaluateEndToEndStep(
	channels []models.Channel,
	aiAgents []models.AIAgent,
	readyAIAgentMap map[int64]bool,
	activeTeamMap map[int64]models.AgentTeam,
	activeScheduleNowTeamSet map[int64]bool,
	readyOperatorTeams map[int64]bool,
) (response.SetupStepItem, bool) {
	issues := make([]response.SetupIssueItem, 0)
	quickFixes := make([]string, 0)
	readyChannelCount := 0
	activeChannelCount := 0

	for _, ch := range channels {
		if ch.Status != enums.StatusOk {
			continue
		}
		activeChannelCount++
		if ch.AIAgentRolloutPercent > 0 && readyAIAgentMap[ch.AIAgentID] {
			readyChannelCount++
		}
	}

	canReceiveNow := readyChannelCount > 0
	status := "ok"
	summaryCode := "ok"

	if !canReceiveNow {
		status = "error"
		summaryCode = "no_end_to_end_path"
		issues = append(issues, response.SetupIssueItem{
			Code:           "no_end_to_end_path",
			Severity:       "error",
			EntityType:     "system",
			QuickFixAction: QuickFixActionQuickSetupAll,
		})
		quickFixes = append(quickFixes, QuickFixActionQuickSetupAll)
	}

	_ = aiAgents
	_ = activeTeamMap
	_ = activeScheduleNowTeamSet
	_ = readyOperatorTeams

	return response.SetupStepItem{
		Key:             "end_to_end",
		StepNumber:      6,
		Status:          status,
		SummaryCode:     summaryCode,
		TotalCount:      max(activeChannelCount, 1),
		ReadyCount:      readyChannelCount,
		Issues:          issues,
		QuickFixActions: quickFixes,
		ManagePath:      "/dashboard/setup",
	}, canReceiveNow
}

func (s *setupReadinessService) evaluateCurrentOperator(
	operator *dto.AuthPrincipal,
	profiles []models.AgentProfile,
	activeTeamMap map[int64]models.AgentTeam,
	activeScheduleNowTeamSet map[int64]bool,
	activeCounts map[int64]int,
) response.CurrentOperatorDiagnostic {
	if operator == nil || operator.UserID <= 0 {
		return response.CurrentOperatorDiagnostic{
			BlockerCodes: []string{"not_authenticated"},
		}
	}

	diag := response.CurrentOperatorDiagnostic{
		UserID:       operator.UserID,
		Username:     operator.Username,
		Nickname:     operator.Nickname,
		BlockerCodes: make([]string, 0),
	}

	var matched *models.AgentProfile
	for i := range profiles {
		if profiles[i].UserID == operator.UserID {
			matched = &profiles[i]
			break
		}
	}

	if matched == nil {
		diag.BlockerCodes = append(diag.BlockerCodes, "missing_profile")
		return diag
	}

	diag.HasProfile = true
	diag.ProfileStatusOk = matched.Status == enums.StatusOk
	diag.TeamID = matched.TeamID
	diag.ServiceStatus = int(matched.ServiceStatus)
	diag.AutoAssignEnabled = matched.AutoAssignEnabled
	diag.ActiveConversationCount = int64(activeCounts[operator.UserID])
	diag.MaxConcurrentCount = matched.MaxConcurrentCount

	if !diag.ProfileStatusOk {
		diag.BlockerCodes = append(diag.BlockerCodes, "profile_disabled")
	}

	if team, ok := activeTeamMap[matched.TeamID]; ok {
		diag.TeamName = team.Name
		diag.TeamActive = true
		diag.TeamHasScheduleNow = activeScheduleNowTeamSet[team.ID]
		if !diag.TeamHasScheduleNow {
			diag.BlockerCodes = append(diag.BlockerCodes, "team_off_hours")
		}
	} else {
		diag.BlockerCodes = append(diag.BlockerCodes, "no_team")
	}

	if !matched.AutoAssignEnabled {
		diag.BlockerCodes = append(diag.BlockerCodes, "auto_assign_disabled")
	}
	if matched.ServiceStatus != enums.ServiceStatusIdle {
		diag.BlockerCodes = append(diag.BlockerCodes, "status_busy")
	}
	if matched.MaxConcurrentCount <= 0 || int(diag.ActiveConversationCount) >= matched.MaxConcurrentCount {
		diag.BlockerCodes = append(diag.BlockerCodes, "at_capacity")
	}

	diag.CanReceiveNow = len(diag.BlockerCodes) == 0
	return diag
}

func (s *setupReadinessService) SimulateRoute(req request.SetupSimulateRouteRequest, operator *dto.AuthPrincipal) (*response.SetupSimulateRouteResponse, error) {
	_ = operator
	now := time.Now()
	db := sqls.DB()

	var channel models.Channel
	if req.ChannelID > 0 {
		if err := db.First(&channel, req.ChannelID).Error; err != nil {
			return &response.SetupSimulateRouteResponse{
				WillReachOperator: false,
				FinalOutcomeCode:  "blocked_no_channel",
				Hops: []response.SimulatedRouteHop{
					{
						HopKey:         "channel",
						Status:         "error",
						ReasonCode:     "blocked_no_channel",
						QuickFixAction: QuickFixActionEnsureDefaultChannel,
					},
				},
			}, nil
		}
	} else {
		if err := db.Where("status = ?", enums.StatusOk).Order("id ASC").First(&channel).Error; err != nil {
			if err2 := db.Order("id ASC").First(&channel).Error; err2 != nil {
				return &response.SetupSimulateRouteResponse{
					WillReachOperator: false,
					FinalOutcomeCode:  "blocked_no_channel",
					Hops: []response.SimulatedRouteHop{
						{
							HopKey:         "channel",
							Status:         "error",
							ReasonCode:     "blocked_no_channel",
							QuickFixAction: QuickFixActionEnsureDefaultChannel,
						},
					},
				}, nil
			}
		}
	}

	resp := &response.SetupSimulateRouteResponse{
		ChannelID:          channel.ID,
		ChannelName:        channel.Name,
		ChannelType:        channel.ChannelType,
		Hops:               make([]response.SimulatedRouteHop, 0, 5),
		CandidateOperators: make([]response.SimulatedOperatorCandidate, 0),
	}

	// Hop 1: Channel
	if channel.Status != enums.StatusOk {
		resp.FinalOutcomeCode = "blocked_channel_disabled"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "channel",
			Status:         "error",
			ReasonCode:     "blocked_channel_disabled",
			EntityID:       channel.ID,
			EntityName:     channel.Name,
			QuickFixAction: QuickFixActionEnsureDefaultChannel,
		})
		return resp, nil
	}
	if channel.AIAgentRolloutPercent <= 0 {
		resp.FinalOutcomeCode = "blocked_zero_rollout"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "channel",
			Status:         "error",
			ReasonCode:     "blocked_zero_rollout",
			EntityID:       channel.ID,
			EntityName:     channel.Name,
			QuickFixAction: QuickFixActionEnsureDefaultChannel,
		})
		return resp, nil
	}
	if channel.AIAgentID <= 0 {
		resp.FinalOutcomeCode = "blocked_missing_ai_agent"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "channel",
			Status:         "error",
			ReasonCode:     "blocked_missing_ai_agent",
			EntityID:       channel.ID,
			EntityName:     channel.Name,
			QuickFixAction: QuickFixActionEnsureDefaultChannel,
		})
		return resp, nil
	}
	resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
		HopKey:     "channel",
		Status:     "ok",
		ReasonCode: "ok",
		EntityID:   channel.ID,
		EntityName: channel.Name,
	})

	// Hop 2: AI Agent
	var aiAgent models.AIAgent
	if err := db.First(&aiAgent, channel.AIAgentID).Error; err != nil || aiAgent.Status != enums.StatusOk {
		resp.FinalOutcomeCode = "blocked_missing_ai_agent"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "ai_agent",
			Status:         "error",
			ReasonCode:     "blocked_missing_ai_agent",
			EntityID:       channel.AIAgentID,
			QuickFixAction: QuickFixActionFixAIAgentHandoff,
		})
		return resp, nil
	}
	if aiAgent.ServiceMode == enums.IMConversationServiceModeAIOnly {
		resp.FinalOutcomeCode = "blocked_ai_only_mode"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "ai_agent",
			Status:         "error",
			ReasonCode:     "blocked_ai_only_mode",
			EntityID:       aiAgent.ID,
			EntityName:     aiAgent.Name,
			QuickFixAction: QuickFixActionFixAIAgentHandoff,
		})
		return resp, nil
	}
	agentHopStatus := "ok"
	agentReason := "ok_human_only"
	if aiAgent.ServiceMode == enums.IMConversationServiceModeAIFirst {
		agentReason = "ok_ai_first_handoff"
		if !aiAgent.AutoHandoffOnFailure {
			agentHopStatus = "warning"
			agentReason = "warning_auto_handoff_off"
		}
	}
	resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
		HopKey:         "ai_agent",
		Status:         agentHopStatus,
		ReasonCode:     agentReason,
		EntityID:       aiAgent.ID,
		EntityName:     aiAgent.Name,
		QuickFixAction: QuickFixActionFixAIAgentHandoff,
	})

	// Hop 3: Target Teams
	requestedTeamIDs := utils.SplitInt64s(aiAgent.TeamIDs)
	var teams []models.AgentTeam
	if len(requestedTeamIDs) > 0 {
		db.Where("id IN ? AND status = ?", requestedTeamIDs, enums.StatusOk).Find(&teams)
	}
	if len(teams) == 0 {
		resp.FinalOutcomeCode = "blocked_agent_no_teams"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "teams",
			Status:         "error",
			ReasonCode:     "blocked_agent_no_teams",
			EntityID:       aiAgent.ID,
			EntityName:     aiAgent.Name,
			QuickFixAction: QuickFixActionFixAIAgentHandoff,
		})
		return resp, nil
	}

	teamNames := make([]string, 0, len(teams))
	enabledTeamIDSet := make(map[int64]models.AgentTeam, len(teams))
	enabledTeamIDs := make([]int64, 0, len(teams))
	for _, t := range teams {
		teamNames = append(teamNames, t.Name)
		enabledTeamIDSet[t.ID] = t
		enabledTeamIDs = append(enabledTeamIDs, t.ID)
	}
	resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
		HopKey:     "teams",
		Status:     "ok",
		ReasonCode: "ok",
		EntityName: strings.Join(teamNames, ", "),
	})

	// Hop 4: Shift Schedules (Active Now)
	activeScheduleTeamIDs := ConversationDispatchService.findActiveScheduleTeamIDs(enabledTeamIDs, now)
	activeScheduleTeamSet := make(map[int64]bool, len(activeScheduleTeamIDs))
	activeScheduleNames := make([]string, 0, len(activeScheduleTeamIDs))
	for _, tid := range activeScheduleTeamIDs {
		activeScheduleTeamSet[tid] = true
		if t, ok := enabledTeamIDSet[tid]; ok {
			activeScheduleNames = append(activeScheduleNames, t.Name)
		}
	}

	if len(activeScheduleTeamIDs) == 0 {
		resp.FinalOutcomeCode = "queued_off_hours_no_schedule"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "schedules",
			Status:         "error",
			ReasonCode:     "queued_off_hours_no_schedule",
			EntityName:     strings.Join(teamNames, ", "),
			QuickFixAction: QuickFixActionEnsureTeamSchedule,
		})
		resp.CandidateOperators = s.buildCandidateListForTeams(enabledTeamIDSet, activeScheduleTeamSet)
		return resp, nil
	}
	resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
		HopKey:     "schedules",
		Status:     "ok",
		ReasonCode: "ok",
		EntityName: strings.Join(activeScheduleNames, ", "),
	})

	// Hop 5: Operator Candidate Selection
	candidates := s.buildCandidateListForTeams(enabledTeamIDSet, activeScheduleTeamSet)
	resp.CandidateOperators = candidates

	eligibleCandidates, _, err := ConversationDispatchService.pickDispatchCandidates(enabledTeamIDs, now)
	if err != nil {
		return nil, err
	}
	if len(eligibleCandidates) == 0 {
		resp.FinalOutcomeCode = "queued_no_eligible_operator"
		resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
			HopKey:         "operators",
			Status:         "error",
			ReasonCode:     "queued_no_eligible_operator",
			QuickFixAction: QuickFixActionEnableMyProfile,
		})
		return resp, nil
	}

	winner := eligibleCandidates[0]
	winnerTeamName := ""
	if t, ok := enabledTeamIDSet[winner.profile.TeamID]; ok {
		winnerTeamName = t.Name
	}
	winnerDisplayName := firstNonEmpty(winner.profile.DisplayName, fmt.Sprintf("#%d", winner.profile.UserID))
	matched := &response.SimulatedOperatorCandidate{
		UserID:             winner.profile.UserID,
		DisplayName:        winnerDisplayName,
		TeamID:             winner.profile.TeamID,
		TeamName:           winnerTeamName,
		Eligible:           true,
		ActiveCount:        int64(winner.activeCount),
		MaxConcurrentCount: winner.profile.MaxConcurrentCount,
		ServiceStatus:      int(winner.profile.ServiceStatus),
		AutoAssignEnabled:  winner.profile.AutoAssignEnabled,
	}
	resp.MatchedOperator = matched
	resp.WillReachOperator = true
	resp.FinalOutcomeCode = "assigned_to_operator"
	resp.Hops = append(resp.Hops, response.SimulatedRouteHop{
		HopKey:     "operators",
		Status:     "ok",
		ReasonCode: "assigned_to_operator",
		EntityID:   winner.profile.UserID,
		EntityName: winnerDisplayName,
	})

	return resp, nil
}

func (s *setupReadinessService) buildCandidateListForTeams(
	enabledTeamIDSet map[int64]models.AgentTeam,
	activeScheduleTeamSet map[int64]bool,
) []response.SimulatedOperatorCandidate {
	db := sqls.DB()
	var allProfiles []models.AgentProfile
	db.Order("id ASC").Find(&allProfiles)

	var allTeams []models.AgentTeam
	db.Find(&allTeams)
	teamNameMap := make(map[int64]string, len(allTeams))
	for _, t := range allTeams {
		teamNameMap[t.ID] = t.Name
	}

	var users []models.User
	db.Find(&users)
	userMap := make(map[int64]models.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	activeCounts, _ := ConversationDispatchService.findActiveConversationCountMap(extractProfileUserIDs(allProfiles))
	result := make([]response.SimulatedOperatorCandidate, 0, len(allProfiles))

	for _, p := range allProfiles {
		u, hasUser := userMap[p.UserID]
		displayName := strings.TrimSpace(p.DisplayName)
		if displayName == "" && hasUser {
			displayName = firstNonEmpty(u.Nickname, u.Username)
		}
		if displayName == "" {
			displayName = fmt.Sprintf("#%d", p.UserID)
		}

		skipReason := ""
		switch {
		case p.Status != enums.StatusOk:
			skipReason = "profile_disabled"
		case !hasUser || u.Status != enums.StatusOk || u.DeletedAt != nil:
			skipReason = "user_disabled"
		case enabledTeamIDSet[p.TeamID].ID == 0:
			skipReason = "team_not_in_agent"
		case !activeScheduleTeamSet[p.TeamID]:
			skipReason = "team_off_hours"
		case !p.AutoAssignEnabled:
			skipReason = "auto_assign_off"
		case p.ServiceStatus != enums.ServiceStatusIdle:
			skipReason = "status_busy"
		case p.MaxConcurrentCount <= 0 || activeCounts[p.UserID] >= p.MaxConcurrentCount:
			skipReason = "at_capacity"
		}

		result = append(result, response.SimulatedOperatorCandidate{
			UserID:             p.UserID,
			DisplayName:        displayName,
			TeamID:             p.TeamID,
			TeamName:           teamNameMap[p.TeamID],
			Eligible:           skipReason == "",
			SkipReasonCode:     skipReason,
			ActiveCount:        int64(activeCounts[p.UserID]),
			MaxConcurrentCount: p.MaxConcurrentCount,
			ServiceStatus:      int(p.ServiceStatus),
			AutoAssignEnabled:  p.AutoAssignEnabled,
		})
	}
	return result
}

func (s *setupReadinessService) ApplyQuickFix(req request.SetupQuickFixRequest, operator *dto.AuthPrincipal) (*response.SetupQuickFixResponse, error) {
	if operator == nil || operator.UserID <= 0 {
		return nil, errorsx.UnauthorizedI18n("error.setup.authRequired")
	}

	action := strings.TrimSpace(req.Action)
	applied := make([]string, 0, 4)

	switch action {
	case QuickFixActionQuickSetupAll:
		a1, err := s.ensureTeamAndSchedule(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a1...)

		a2, err := s.enableOperatorProfile(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a2...)

		a3, err := s.fixAIAgentHandoff(req.EntityID, operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a3...)

		a4, err := s.ensureDefaultChannel(req.EntityID, operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a4...)

	case QuickFixActionEnsureTeamSchedule:
		a1, err := s.ensureTeamAndSchedule(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a1...)

	case QuickFixActionEnableMyProfile:
		a1, err := s.ensureTeamAndSchedule(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a1...)

		a2, err := s.enableOperatorProfile(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a2...)

	case QuickFixActionFixAIAgentHandoff:
		a1, err := s.ensureTeamAndSchedule(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a1...)

		a2, err := s.fixAIAgentHandoff(req.EntityID, operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a2...)

	case QuickFixActionEnsureDefaultChannel:
		a1, err := s.ensureTeamAndSchedule(operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a1...)

		a2, err := s.fixAIAgentHandoff(0, operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a2...)

		a3, err := s.ensureDefaultChannel(req.EntityID, operator)
		if err != nil {
			return nil, err
		}
		applied = append(applied, a3...)

	default:
		return nil, errorsx.InvalidParamI18n("error.setup.invalidQuickFixAction", action)
	}

	// Trigger dispatch of any pending conversations now that routing is fixed
	_, _ = ConversationDispatchService.DispatchPendingConversations(0)

	readiness, err := s.GetReadiness(operator)
	if err != nil {
		return nil, err
	}

	return &response.SetupQuickFixResponse{
		AppliedActions: applied,
		Readiness:      *readiness,
	}, nil
}

func (s *setupReadinessService) ensureTeamAndSchedule(operator *dto.AuthPrincipal) ([]string, error) {
	now := time.Now()
	db := sqls.DB()
	applied := make([]string, 0, 2)

	var activeTeams []models.AgentTeam
	if err := db.Where("status = ?", enums.StatusOk).Order("id ASC").Find(&activeTeams).Error; err != nil {
		return nil, err
	}

	if len(activeTeams) == 0 {
		var anyTeam models.AgentTeam
		if err := db.Order("id ASC").First(&anyTeam).Error; err == nil && anyTeam.ID > 0 {
			anyTeam.Status = enums.StatusOk
			anyTeam.UpdatedAt = now
			anyTeam.UpdateUserID = operator.UserID
			anyTeam.UpdateUserName = operator.Username
			if err := db.Save(&anyTeam).Error; err != nil {
				return nil, err
			}
			activeTeams = append(activeTeams, anyTeam)
			applied = append(applied, "enabled_team")
		} else {
			newTeam := models.AgentTeam{
				Name:         "Default Support Team",
				LeaderUserID: operator.UserID,
				Status:       enums.StatusOk,
				Description:  "Auto-created support team",
				AuditFields: models.AuditFields{
					CreatedAt:      now,
					CreateUserID:   operator.UserID,
					CreateUserName: operator.Username,
					UpdatedAt:      now,
					UpdateUserID:   operator.UserID,
					UpdateUserName: operator.Username,
				},
			}
			if err := db.Create(&newTeam).Error; err != nil {
				return nil, err
			}
			activeTeams = append(activeTeams, newTeam)
			applied = append(applied, "created_default_team")
		}
	}

	for _, team := range activeTeams {
		var count int64
		if err := db.Model(&models.AgentTeamSchedule{}).
			Where("team_id = ? AND status = ? AND start_at <= ? AND end_at > ?", team.ID, enums.StatusOk, now, now).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			schedule := models.AgentTeamSchedule{
				TeamID:  team.ID,
				StartAt: now.Add(-1 * time.Hour),
				EndAt:   now.AddDate(1, 0, 0),
				Remark:  "24/7 Continuous Shift (Auto-Setup)",
				Status:  enums.StatusOk,
				AuditFields: models.AuditFields{
					CreatedAt:      now,
					CreateUserID:   operator.UserID,
					CreateUserName: operator.Username,
					UpdatedAt:      now,
					UpdateUserID:   operator.UserID,
					UpdateUserName: operator.Username,
				},
			}
			if err := db.Create(&schedule).Error; err != nil {
				return nil, err
			}
			applied = append(applied, "created_active_schedule")
		}
	}

	return applied, nil
}

func (s *setupReadinessService) enableOperatorProfile(operator *dto.AuthPrincipal) ([]string, error) {
	now := time.Now()
	db := sqls.DB()
	applied := make([]string, 0, 1)

	var firstTeam models.AgentTeam
	if err := db.Where("status = ?", enums.StatusOk).Order("id ASC").First(&firstTeam).Error; err != nil {
		return nil, err
	}

	var profile models.AgentProfile
	err := db.Where("user_id = ?", operator.UserID).First(&profile).Error
	if err != nil {
		displayName := firstNonEmpty(operator.Nickname, operator.Username, fmt.Sprintf("Agent-%d", operator.UserID))
		newProfile := models.AgentProfile{
			UserID:                operator.UserID,
			TeamID:                firstTeam.ID,
			AgentCode:             fmt.Sprintf("AG%04d", operator.UserID),
			DisplayName:           displayName,
			ServiceStatus:         enums.ServiceStatusIdle,
			MaxConcurrentCount:    10,
			PriorityLevel:         1,
			AutoAssignEnabled:     true,
			ReceiveOfflineMessage: true,
			LastOnlineAt:          &now,
			LastStatusAt:          &now,
			Status:                enums.StatusOk,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   operator.UserID,
				CreateUserName: operator.Username,
				UpdatedAt:      now,
				UpdateUserID:   operator.UserID,
				UpdateUserName: operator.Username,
			},
		}
		if err := db.Create(&newProfile).Error; err != nil {
			return nil, err
		}
		applied = append(applied, "created_operator_profile")
		return applied, nil
	}

	targetTeamID := profile.TeamID
	var teamCheck models.AgentTeam
	if targetTeamID <= 0 || db.Where("id = ? AND status = ?", targetTeamID, enums.StatusOk).First(&teamCheck).Error != nil {
		targetTeamID = firstTeam.ID
	}

	maxConcurrent := profile.MaxConcurrentCount
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	updates := map[string]any{
		"team_id":              targetTeamID,
		"service_status":       enums.ServiceStatusIdle,
		"auto_assign_enabled":  true,
		"max_concurrent_count": maxConcurrent,
		"status":               enums.StatusOk,
		"last_online_at":       now,
		"last_status_at":       now,
		"updated_at":           now,
		"update_user_id":       operator.UserID,
		"update_user_name":     operator.Username,
	}
	if err := db.Model(&models.AgentProfile{}).Where("id = ?", profile.ID).Updates(updates).Error; err != nil {
		return nil, err
	}
	applied = append(applied, "updated_operator_profile_online")
	return applied, nil
}

func (s *setupReadinessService) fixAIAgentHandoff(targetAgentID int64, operator *dto.AuthPrincipal) ([]string, error) {
	now := time.Now()
	db := sqls.DB()
	applied := make([]string, 0, 2)

	var activeTeams []models.AgentTeam
	if err := db.Where("status = ?", enums.StatusOk).Order("id ASC").Find(&activeTeams).Error; err != nil {
		return nil, err
	}
	teamIDStrs := make([]string, 0, len(activeTeams))
	activeTeamSet := make(map[int64]bool, len(activeTeams))
	for _, t := range activeTeams {
		teamIDStrs = append(teamIDStrs, strconv.FormatInt(t.ID, 10))
		activeTeamSet[t.ID] = true
	}
	defaultTeamIDsCSV := strings.Join(teamIDStrs, ",")

	var agents []models.AIAgent
	if targetAgentID > 0 {
		db.Where("id = ?", targetAgentID).Find(&agents)
	} else {
		db.Order("id ASC").Find(&agents)
	}

	if len(agents) == 0 {
		newAgent := models.AIAgent{
			Name:                 "Default Reception Agent",
			Description:          "Routes customer inquiries to human operators",
			Status:               enums.StatusOk,
			ServiceMode:          enums.IMConversationServiceModeHumanOnly,
			RolloutPercent:       100,
			TeamIDs:              defaultTeamIDsCSV,
			HandoffMode:          enums.AIAgentHandoffModeDefaultTeamPool,
			FallbackMode:         enums.AIAgentFallbackModeHandoff,
			AutoHandoffOnFailure: true,
			ReplyTimeoutSeconds:  180,
			MaxSteps:             6,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   operator.UserID,
				CreateUserName: operator.Username,
				UpdatedAt:      now,
				UpdateUserID:   operator.UserID,
				UpdateUserName: operator.Username,
			},
		}
		if err := db.Create(&newAgent).Error; err != nil {
			return nil, err
		}
		applied = append(applied, "created_default_ai_agent")
		return applied, nil
	}

	for _, agent := range agents {
		existingTeamIDs := utils.SplitInt64s(agent.TeamIDs)
		validTeamIDs := make([]string, 0, len(existingTeamIDs))
		for _, tid := range existingTeamIDs {
			if activeTeamSet[tid] {
				validTeamIDs = append(validTeamIDs, strconv.FormatInt(tid, 10))
			}
		}
		newTeamCSV := strings.Join(validTeamIDs, ",")
		if newTeamCSV == "" {
			newTeamCSV = defaultTeamIDsCSV
		}

		hasValidAIConfig := false
		if agent.AIConfigID > 0 {
			var aiCfg models.AIConfig
			if err := db.Where("id = ? AND status = ?", agent.AIConfigID, enums.StatusOk).First(&aiCfg).Error; err == nil && aiCfg.ID > 0 {
				hasValidAIConfig = true
			}
		}

		newServiceMode := agent.ServiceMode
		if !hasValidAIConfig {
			newServiceMode = enums.IMConversationServiceModeHumanOnly
		} else if newServiceMode == enums.IMConversationServiceModeAIOnly || newServiceMode == 0 {
			newServiceMode = enums.IMConversationServiceModeAIFirst
		}

		rollout := agent.RolloutPercent
		if rollout <= 0 {
			rollout = 100
		}

		updates := map[string]any{
			"status":                  enums.StatusOk,
			"team_ids":                newTeamCSV,
			"service_mode":            newServiceMode,
			"auto_handoff_on_failure": true,
			"rollout_percent":         rollout,
			"updated_at":              now,
			"update_user_id":          operator.UserID,
			"update_user_name":        operator.Username,
		}
		if err := db.Model(&models.AIAgent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		applied = append(applied, "updated_ai_agent_handoff")
	}

	return applied, nil
}

func (s *setupReadinessService) ensureDefaultChannel(targetChannelID int64, operator *dto.AuthPrincipal) ([]string, error) {
	now := time.Now()
	db := sqls.DB()
	applied := make([]string, 0, 1)

	var firstAgent models.AIAgent
	if err := db.Where("status = ?", enums.StatusOk).Order("id ASC").First(&firstAgent).Error; err != nil {
		return nil, err
	}

	var channels []models.Channel
	if targetChannelID > 0 {
		db.Where("id = ?", targetChannelID).Find(&channels)
	} else {
		db.Order("id ASC").Find(&channels)
	}

	if len(channels) == 0 {
		newChannel := models.Channel{
			Name:                  "Default Web Support Channel",
			ChannelType:           string(enums.ChannelTypeWeb),
			ChannelID:             fmt.Sprintf("web_default_%d", now.Unix()),
			AIAgentID:             firstAgent.ID,
			AIAgentRolloutPercent: 100,
			ConfigJSON:            `{"position":"right"}`,
			Status:                enums.StatusOk,
			AuditFields: models.AuditFields{
				CreatedAt:      now,
				CreateUserID:   operator.UserID,
				CreateUserName: operator.Username,
				UpdatedAt:      now,
				UpdateUserID:   operator.UserID,
				UpdateUserName: operator.Username,
			},
		}
		if err := db.Create(&newChannel).Error; err != nil {
			return nil, err
		}
		applied = append(applied, "created_default_channel")
		return applied, nil
	}

	for _, ch := range channels {
		agentID := ch.AIAgentID
		var checkAgent models.AIAgent
		if agentID <= 0 || db.Where("id = ? AND status = ?", agentID, enums.StatusOk).First(&checkAgent).Error != nil {
			agentID = firstAgent.ID
		}
		rollout := ch.AIAgentRolloutPercent
		if rollout <= 0 {
			rollout = 100
		}
		updates := map[string]any{
			"status":                   enums.StatusOk,
			"ai_agent_id":              agentID,
			"ai_agent_rollout_percent": rollout,
			"updated_at":               now,
			"update_user_id":           operator.UserID,
			"update_user_name":         operator.Username,
		}
		if err := db.Model(&models.Channel{}).Where("id = ?", ch.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		applied = append(applied, "updated_channel_routing")
	}

	return applied, nil
}

func extractProfileUserIDs(profiles []models.AgentProfile) []int64 {
	ids := make([]int64, 0, len(profiles))
	seen := make(map[int64]bool, len(profiles))
	for _, p := range profiles {
		if p.UserID > 0 && !seen[p.UserID] {
			seen[p.UserID] = true
			ids = append(ids, p.UserID)
		}
	}
	return ids
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

var (
	_ = math.Max
	_ = slices.SortFunc[[]int, int]
)
