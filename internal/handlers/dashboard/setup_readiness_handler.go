package dashboard

import (
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

func SetupGetReadiness(ctx *gin.Context) {
	principal, err := services.AuthService.Authenticate(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	res, err := services.SetupReadinessService.GetReadiness(principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, res)
}

func SetupPostSimulate_route(ctx *gin.Context) {
	principal, err := services.AuthService.Authenticate(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.SetupSimulateRouteRequest{}
	if ctx.Request != nil && ctx.Request.ContentLength > 0 {
		if err := params.ReadJSON(ctx, &req); err != nil {
			httpx.WriteJSON(ctx, err)
			return
		}
	}
	res, err := services.SetupReadinessService.SimulateRoute(req, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, res)
}

func SetupPostQuick_fix(ctx *gin.Context) {
	principal, err := services.AuthService.Authenticate(ctx)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := request.SetupQuickFixRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	res, err := services.SetupReadinessService.ApplyQuickFix(req, principal)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, res)
}
