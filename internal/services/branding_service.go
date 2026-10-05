package services

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"agent-desk/internal/pkg/config"
	"agent-desk/internal/pkg/dto"
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

// Branding is the deployment's own name and logo, resolved for display.
type Branding struct {
	CompanyName    string
	CompanyLogoURL string
}

// brandingCacheTTL bounds how stale the resolved branding may be. The value is
// read on every page load through /api/config, so it is cached rather than
// queried each time, and a short TTL keeps a multi-instance deployment in step
// without a reload.
const brandingCacheTTL = 30 * time.Second

var (
	brandingCacheMu  sync.RWMutex
	brandingCache    Branding
	brandingCachedAt time.Time
)

// ResolveBranding returns the name and logo every surface should render.
//
// Precedence is deliberate: what an operator saved in the dashboard wins, then
// the configured defaults, then the translated product name. Configuration stays
// in the chain so a deployment that already sets COMPANY_NAME keeps its branding
// the moment this ships, without anyone copying it into the database.
//
// It never fails. This feeds the public /api/config endpoint, so a database
// problem must degrade to the configured or default name rather than take the
// whole interface down with it.
func ResolveBranding() Branding {
	brandingCacheMu.RLock()
	if !brandingCachedAt.IsZero() && time.Since(brandingCachedAt) < brandingCacheTTL {
		cached := brandingCache
		brandingCacheMu.RUnlock()
		return cached
	}
	brandingCacheMu.RUnlock()

	resolved := loadBranding()

	brandingCacheMu.Lock()
	brandingCache = resolved
	brandingCachedAt = time.Now()
	brandingCacheMu.Unlock()
	return resolved
}

// InvalidateBrandingCache drops the cached value so the next read goes back to
// the database. Called after a save.
func InvalidateBrandingCache() {
	brandingCacheMu.Lock()
	brandingCachedAt = time.Time{}
	brandingCacheMu.Unlock()
}

func loadBranding() Branding {
	var configuredName, configuredLogo string
	if current := config.GetCurrent(); current != nil {
		configuredName = strings.TrimSpace(current.Server.CompanyName)
		configuredLogo = strings.TrimSpace(current.Server.CompanyLogoURL)
	}

	branding := Branding{CompanyName: configuredName, CompanyLogoURL: configuredLogo}
	if name := readBrandingString(systemConfigKeyBrandingCompanyName); name != "" {
		branding.CompanyName = name
	}
	if logo := readBrandingString(systemConfigKeyBrandingCompanyLogo); logo != "" {
		branding.CompanyLogoURL = logo
	}
	if branding.CompanyName == "" {
		branding.CompanyName = defaultBrandingCompanyName()
	}
	return branding
}

// readBrandingString returns the saved value, or an empty string when nothing is
// stored, the database is unavailable, or the stored value cannot be read.
func readBrandingString(key string) string {
	if sqls.DB() == nil {
		return ""
	}
	item := repositories.SystemConfigRepository.FindByGroupAndKey(
		sqls.DB(), systemConfigGroupBranding, key)
	if item == nil {
		return ""
	}
	var value string
	if err := json.Unmarshal([]byte(item.ConfigValue), &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func (s *systemConfigService) GetDashboardBranding() response.DashboardBrandingResponse {
	savedName := readBrandingString(systemConfigKeyBrandingCompanyName)
	savedLogo := readBrandingString(systemConfigKeyBrandingCompanyLogo)

	var configuredName, configuredLogo string
	if current := config.GetCurrent(); current != nil {
		configuredName = strings.TrimSpace(current.Server.CompanyName)
		configuredLogo = strings.TrimSpace(current.Server.CompanyLogoURL)
	}

	return response.DashboardBrandingResponse{
		// The form shows what is actually in effect, not only what is stored, so an
		// operator who has never opened this page sees the real current state.
		CompanyName:            firstNonBlank(savedName, configuredName),
		CompanyLogoURL:         firstNonBlank(savedLogo, configuredLogo),
		SavedCompanyName:       savedName,
		SavedCompanyLogoURL:    savedLogo,
		ConfiguredCompanyName:  configuredName,
		ConfiguredCompanyLogo:  configuredLogo,
		EffectiveCompanyName:   ResolveBranding().CompanyName,
		UsesConfiguredFallback: savedName == "" && configuredName != "",
	}
}

// SaveBranding stores the operator's values. An empty string clears the saved
// value and hands control back to configuration, which is how an operator undoes
// an override without deleting rows by hand.
func (s *systemConfigService) SaveBranding(payload map[string]json.RawMessage, operator *dto.AuthPrincipal) (response.DashboardBrandingResponse, error) {
	if err := s.SaveGroupConfig(systemConfigGroupBranding, payload, operator); err != nil {
		return response.DashboardBrandingResponse{}, err
	}
	InvalidateBrandingCache()
	return s.GetDashboardBranding(), nil
}
