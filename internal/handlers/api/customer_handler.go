package api

import (
	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/openidentity"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
)

func CustomerPostSession_exchange(ctx *gin.Context) {
	channel := services.ChannelService.GetEnabledChannel(ctx)
	if channel == nil {
		httpx.WriteJSON(ctx, httpx.JsonErrorMsg(ctx, "error.e0209"))
		return
	}
	externalUser, err := openidentity.GetExternalUser(ctx, openidentity.Secrets{
		UserToken:    services.ChannelService.GetUserTokenSecret(channel),
		IdentityHMAC: config.Current().Identity.Secret,
		ChannelID:    channel.ChannelID,
	})
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	resp, err := services.CustomerSessionService.Exchange(channel, *externalUser)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, resp)
}
