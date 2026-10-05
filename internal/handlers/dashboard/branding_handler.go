package dashboard

import (
	"encoding/json"

	"agent-desk/internal/pkg/constants"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/httpx"
	"agent-desk/internal/pkg/httpx/params"
	"agent-desk/internal/pkg/i18nx"
	"agent-desk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/mlogclub/simple/web"
)

func BrandingGetConfig(ctx *gin.Context) {
	if _, err := services.AuthService.RequirePermission(ctx, constants.PermissionBrandingView); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, services.SystemConfigService.GetDashboardBranding())
}

// BrandingPostSave renames the deployment. The name and logo apply to the
// dashboard, the visitor portal and the widget chrome alike, because all three
// read the same resolved branding.
func BrandingPostSave(ctx *gin.Context) {
	operator, err := services.AuthService.RequirePermission(ctx, constants.PermissionBrandingUpdate)
	if err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}
	req := map[string]json.RawMessage{}
	if err := params.ReadJSON(ctx, &req); err != nil {
		httpx.WriteJSON(ctx, err)
		return
	}

	branding, err := services.SystemConfigService.SaveBranding(req, operator)
	if err != nil {
		if validationErr, ok := err.(*services.SystemConfigValidationError); ok {
			locale := i18nx.Locale(ctx)
			httpx.WriteJSON(ctx, web.JsonErrorData(errorsx.CodeInvalidParam, validationErr.Message(locale), gin.H{
				"errors": validationErr.FieldErrorsLocale(locale),
			}))
			return
		}
		httpx.WriteJSON(ctx, err)
		return
	}
	httpx.WriteJSON(ctx, branding)
}
