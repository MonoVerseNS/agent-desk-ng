package openidentity

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-desk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
)

func TestSignAndVerifyExternalIDSignature(t *testing.T) {
	const secret = "shared-identity-secret"
	const channelID = "web-main"
	const externalID = "u_10001"

	signature := SignExternalID(secret, channelID, externalID)
	if len(signature) != 64 {
		t.Fatalf("signature = %q, want 64 hex chars", signature)
	}
	if !VerifyExternalIDSignature(secret, channelID, externalID, signature) {
		t.Fatal("expected a freshly produced signature to verify")
	}
	if strings.ToLower(signature) != signature {
		t.Fatalf("signature = %q, want lower-case hex", signature)
	}
}

func TestVerifyExternalIDSignatureRejects(t *testing.T) {
	const secret = "shared-identity-secret"
	const channelID = "web-main"
	const externalID = "u_10001"
	valid := SignExternalID(secret, channelID, externalID)

	tests := []struct {
		name       string
		secret     string
		channelID  string
		externalID string
		signature  string
	}{
		{name: "wrong secret", secret: "other-secret", channelID: channelID, externalID: externalID, signature: valid},
		{name: "empty secret disables the feature", secret: "", channelID: channelID, externalID: externalID, signature: valid},
		{name: "signature from another channel", secret: secret, channelID: "web-other", externalID: externalID, signature: valid},
		{name: "signature for another visitor", secret: secret, channelID: channelID, externalID: "u_20002", signature: valid},
		{name: "truncated signature", secret: secret, channelID: channelID, externalID: externalID, signature: valid[:len(valid)-2]},
		{name: "single flipped nibble", secret: secret, channelID: channelID, externalID: externalID, signature: flipLastNibble(valid)},
		{name: "not hex", secret: secret, channelID: channelID, externalID: externalID, signature: strings.Repeat("z", 64)},
		{name: "empty signature", secret: secret, channelID: channelID, externalID: externalID, signature: ""},
		{name: "empty external id", secret: secret, channelID: channelID, externalID: "", signature: valid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if VerifyExternalIDSignature(tt.secret, tt.channelID, tt.externalID, tt.signature) {
				t.Fatal("expected signature to be rejected")
			}
		})
	}
}

// A plain embed passes an unsigned external ID. It must keep working, but the
// visitor must land as an anonymous guest rather than as an identified user.
func TestGetExternalUserUnsignedIDIsGuest(t *testing.T) {
	ctx, _ := newIdentityContext(http.MethodPost, map[string]string{
		"X-Channel-Id":    "web-main",
		"X-External-Id":   "guest_abc",
		"X-External-Name": "Ivan",
	})

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: "shared-secret", ChannelID: "web-main"})
	if err != nil {
		t.Fatalf("unsigned embed must not be rejected: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceGuest {
		t.Fatalf("ExternalSource = %q, want %q", user.ExternalSource, enums.ExternalSourceGuest)
	}
	if user.ExternalID != "guest_abc" {
		t.Fatalf("ExternalID = %q, want guest_abc", user.ExternalID)
	}
	if user.ExternalName != "Ivan" {
		t.Fatalf("ExternalName = %q, want Ivan", user.ExternalName)
	}
}

func TestGetExternalUserSignedIDIsIdentified(t *testing.T) {
	const secret = "shared-secret"
	const channelID = "web-main"
	const externalID = "u_10001"

	ctx, _ := newIdentityContext(http.MethodPost, map[string]string{
		"X-External-Id":           externalID,
		"X-External-Id-Signature": SignExternalID(secret, channelID, externalID),
	})

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: secret, ChannelID: channelID})
	if err != nil {
		t.Fatalf("signed id must be accepted: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceUser {
		t.Fatalf("ExternalSource = %q, want %q", user.ExternalSource, enums.ExternalSourceUser)
	}
	if user.ExternalID != externalID {
		t.Fatalf("ExternalID = %q, want %q", user.ExternalID, externalID)
	}
}

// The core impersonation case: someone replays a valid signature against a
// different visitor ID.
func TestGetExternalUserReplayedSignatureDowngradesToGuest(t *testing.T) {
	const secret = "shared-secret"
	const channelID = "web-main"

	ctx, _ := newIdentityContext(http.MethodPost, map[string]string{
		"X-External-Id":           "victim_999",
		"X-External-Id-Signature": SignExternalID(secret, channelID, "attacker_1"),
	})

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: secret, ChannelID: channelID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceGuest {
		t.Fatalf("ExternalSource = %q, want %q for a replayed signature", user.ExternalSource, enums.ExternalSourceGuest)
	}
}

// Without a configured secret nothing can ever be trusted, which is the safe
// default for an existing deployment.
func TestGetExternalUserWithoutConfiguredSecretIsGuest(t *testing.T) {
	ctx, _ := newIdentityContext(http.MethodPost, map[string]string{
		"X-External-Id":           "u_10001",
		"X-External-Id-Signature": SignExternalID("shared-secret", "web-main", "u_10001"),
	})

	user, err := GetExternalUser(ctx, Secrets{ChannelID: "web-main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceGuest {
		t.Fatalf("ExternalSource = %q, want guest when no secret is configured", user.ExternalSource)
	}
}

func TestGetExternalUserRequiresAnExternalID(t *testing.T) {
	ctx, _ := newIdentityContext(http.MethodPost, map[string]string{"X-Channel-Id": "web-main"})

	if _, err := GetExternalUser(ctx, Secrets{IdentityHMAC: "shared-secret", ChannelID: "web-main"}); err == nil {
		t.Fatal("expected an error when no external id is supplied")
	}
}

func TestGetExternalUserAcceptsSignatureFromQuery(t *testing.T) {
	const secret = "shared-secret"
	const channelID = "web-main"
	const externalID = "u_10001"

	ctx, _ := newIdentityContext(http.MethodGet, nil)
	ctx.Request = httptest.NewRequest(http.MethodGet,
		"/api/customer/session_exchange?externalId="+externalID+"&externalIdSignature="+SignExternalID(secret, channelID, externalID), nil)

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: secret, ChannelID: channelID})
	if err != nil {
		t.Fatalf("signed id from the query string must be accepted: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceUser {
		t.Fatalf("ExternalSource = %q, want %q", user.ExternalSource, enums.ExternalSourceUser)
	}
}

func newIdentityContext(method string, headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, "/api/customer/session_exchange", nil)
	for key, value := range headers {
		ctx.Request.Header.Set(key, value)
	}
	return ctx, recorder
}

func flipLastNibble(signature string) string {
	last := signature[len(signature)-1]
	replacement := byte('0')
	if last == '0' {
		replacement = '1'
	}
	return signature[:len(signature)-1] + string(replacement)
}
