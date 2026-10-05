package services

import (
	"encoding/json"
	"strings"

	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/i18nx"
)

const (
	brandingCompanyNameMaxLength  = 120
	brandingCompanyLogoURLMaxLen  = 500
	brandingDefaultCompanyNameKey = "app.brand"
)

// brandingCompanyNameValidator keeps the platform name a plain single-line
// string. It is used as fallback in headers and document titles, so newlines and
// markup would show up literally.
type brandingCompanyNameValidator struct{}

func (brandingCompanyNameValidator) Validate(raw json.RawMessage) (json.RawMessage, []response.ConfigFieldError, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, []response.ConfigFieldError{
			configFieldError("companyName", "invalid_type", "error.branding.companyNameInvalid"),
		}, nil
	}

	normalized := strings.Join(strings.Fields(value), " ")
	if len([]rune(normalized)) > brandingCompanyNameMaxLength {
		return nil, []response.ConfigFieldError{
			configFieldError("companyName", "too_long", "error.branding.companyNameTooLong"),
		}, nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	return encoded, nil, nil
}

// brandingCompanyLogoURLValidator allows an empty value, which means "keep the
// bundled logo". A supplied value has to be a relative path or an http(s) URL;
// anything else could inject a javascript: scheme into every page that renders
// the logo.
type brandingCompanyLogoURLValidator struct{}

func (brandingCompanyLogoURLValidator) Validate(raw json.RawMessage) (json.RawMessage, []response.ConfigFieldError, error) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, []response.ConfigFieldError{
			configFieldError("companyLogoUrl", "invalid_type", "error.branding.companyLogoUrlInvalid"),
		}, nil
	}

	normalized := strings.TrimSpace(value)
	if normalized == "" {
		encoded, err := json.Marshal("")
		if err != nil {
			return nil, nil, err
		}
		return encoded, nil, nil
	}
	if len([]rune(normalized)) > brandingCompanyLogoURLMaxLen {
		return nil, []response.ConfigFieldError{
			configFieldError("companyLogoUrl", "too_long", "error.branding.companyLogoUrlTooLong"),
		}, nil
	}
	if !isAllowedBrandingLogoURL(normalized) {
		return nil, []response.ConfigFieldError{
			configFieldError("companyLogoUrl", "invalid_url", "error.branding.companyLogoUrlInvalid"),
		}, nil
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, err
	}
	return encoded, nil, nil
}

func isAllowedBrandingLogoURL(value string) bool {
	if strings.HasPrefix(value, "/") {
		// Protocol-relative "//host" would leave the deployment and is not a local asset.
		return !strings.HasPrefix(value, "//")
	}
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "data:image/")
}

// defaultBrandingCompanyName falls back to the translated product name rather
// than to a hardcoded string, so a deployment that never sets a name still reads
// correctly in the visitor's language.
func defaultBrandingCompanyName() string {
	return i18nx.Getf(i18nx.DefaultLocale, brandingDefaultCompanyNameKey)
}
