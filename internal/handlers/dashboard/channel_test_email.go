package dashboard

import (
	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/dto/request"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

// ChannelPostTest_email checks that the channel's bound mail server is reachable
// and will accept its credentials, without sending any mail.
//
// It exists so binding a channel to an SMTP server is a verified action. The
// alternative is finding out that a host, port or password is wrong by filing a
// real request and watching the outbox retry five times.
func ChannelPostTest_email(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionChannelUpdate); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	req := request.IDRequest{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	if req.ID <= 0 {
		httpx.WriteJSON(ctx, errorsx.InvalidParamI18n("error.email.channelIdRequired"))
		return
	}

	// The probe failure message carries the transport detail, which is the
	// actionable part for whoever bound the server.
	result, err := services.EmailDeliveryService.TestChannelSMTP(ctx.Request.Context(), req.ID)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, result)
}
