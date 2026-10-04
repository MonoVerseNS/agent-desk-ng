package openidentity

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"agent-desk/internal/pkg/enums"

	"github.com/gin-gonic/gin"
)

const (
	testSecret    = "shared-identity-secret"
	testChannelID = "web-main"
	testUserID    = "u_10001"
)

func TestSignAndVerifyExternalIDSignature(t *testing.T) {
	now := time.Now()
	signature := SignExternalID(testSecret, testChannelID, testUserID, now)

	if len(signature) != 64 {
		t.Fatalf("signature = %q, want 64 hex chars", signature)
	}
	if strings.ToLower(signature) != signature {
		t.Fatalf("signature = %q, want lower-case hex", signature)
	}
	if !VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, now.Unix(), 24*time.Hour, now) {
		t.Fatal("expected a freshly produced signature to verify")
	}
}

func TestVerifyExternalIDSignatureRejects(t *testing.T) {
	now := time.Now()
	valid := SignExternalID(testSecret, testChannelID, testUserID, now)

	tests := []struct {
		name       string
		secret     string
		channelID  string
		externalID string
		signature  string
		issuedAt   int64
	}{
		{name: "wrong secret", secret: "other-secret", channelID: testChannelID, externalID: testUserID, signature: valid, issuedAt: now.Unix()},
		{name: "empty secret disables the feature", secret: "", channelID: testChannelID, externalID: testUserID, signature: valid, issuedAt: now.Unix()},
		{name: "signature from another channel", secret: testSecret, channelID: "web-other", externalID: testUserID, signature: valid, issuedAt: now.Unix()},
		{name: "signature for another visitor", secret: testSecret, channelID: testChannelID, externalID: "u_20002", signature: valid, issuedAt: now.Unix()},
		{name: "truncated signature", secret: testSecret, channelID: testChannelID, externalID: testUserID, signature: valid[:len(valid)-2], issuedAt: now.Unix()},
		{name: "single flipped nibble", secret: testSecret, channelID: testChannelID, externalID: testUserID, signature: flipLastChar(valid), issuedAt: now.Unix()},
		{name: "not hex", secret: testSecret, channelID: testChannelID, externalID: testUserID, signature: strings.Repeat("z", 64), issuedAt: now.Unix()},
		{name: "empty signature", secret: testSecret, channelID: testChannelID, externalID: testUserID, signature: "", issuedAt: now.Unix()},
		{name: "empty external id", secret: testSecret, channelID: testChannelID, externalID: "", signature: valid, issuedAt: now.Unix()},
		{name: "missing issued at", secret: testSecret, channelID: testChannelID, externalID: testUserID, signature: valid, issuedAt: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if VerifyExternalIDSignature(tt.secret, tt.channelID, tt.externalID, tt.signature, tt.issuedAt, 24*time.Hour, now) {
				t.Fatal("expected signature to be rejected")
			}
		})
	}
}

// The timestamp travels as its own field so the host does not have to encode it
// inside the signature, but it is covered by the signed bytes, so re-dating a
// captured signature breaks verification.
func TestVerifyExternalIDSignatureRejectsRedatedSignature(t *testing.T) {
	now := time.Now()
	signature := SignExternalID(testSecret, testChannelID, testUserID, now)

	// Claim the signature was issued a minute ago.
	if VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, now.Add(-time.Minute).Unix(), 24*time.Hour, now) {
		t.Fatal("expected a re-dated signature to be rejected")
	}
	// Claim it was issued an hour from now to push it past the window.
	if VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, now.Add(time.Hour).Unix(), 24*time.Hour, now) {
		t.Fatal("expected a future-dated signature to be rejected")
	}
}

func TestVerifyExternalIDSignatureExpires(t *testing.T) {
	issuedAt := time.Now().Add(-25 * time.Hour)
	signature := SignExternalID(testSecret, testChannelID, testUserID, issuedAt)

	if VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, issuedAt.Unix(), 24*time.Hour, time.Now()) {
		t.Fatal("expected a signature older than the max age to be rejected")
	}
	if !VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, issuedAt.Unix(), 72*time.Hour, time.Now()) {
		t.Fatal("expected the same signature to be accepted with a longer max age")
	}
}

func TestVerifyExternalIDSignatureToleratesSmallClockSkew(t *testing.T) {
	issuedAt := time.Now().Add(2 * time.Minute)
	signature := SignExternalID(testSecret, testChannelID, testUserID, issuedAt)

	if !VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, issuedAt.Unix(), 24*time.Hour, time.Now()) {
		t.Fatal("expected a small forward clock skew to be tolerated")
	}
}

func TestVerifyExternalIDSignatureDefaultsMaxAge(t *testing.T) {
	issuedAt := time.Now().Add(-(DefaultIdentityMaxAge - time.Hour))
	signature := SignExternalID(testSecret, testChannelID, testUserID, issuedAt)

	if !VerifyExternalIDSignature(testSecret, testChannelID, testUserID, signature, issuedAt.Unix(), 0, time.Now()) {
		t.Fatal("expected the default max age to apply when none is configured")
	}
}

// A plain embed passes an unsigned external ID. It must keep working, but the
// visitor must land as an anonymous guest rather than as an identified user.
func TestGetExternalUserUnsignedIDIsGuest(t *testing.T) {
	ctx, _ := newIdentityContext(map[string]string{
		"X-External-Id":   "guest_abc",
		"X-External-Name": "Ivan",
	})

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, ChannelID: testChannelID})
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
	ctx, _ := newIdentityContext(signedIdentityHeaders(testSecret, testChannelID, testUserID, time.Now()))

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, ChannelID: testChannelID})
	if err != nil {
		t.Fatalf("signed id must be accepted: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceUser {
		t.Fatalf("ExternalSource = %q, want %q", user.ExternalSource, enums.ExternalSourceUser)
	}
	if user.ExternalID != testUserID {
		t.Fatalf("ExternalID = %q, want %q", user.ExternalID, testUserID)
	}
}

// The core impersonation case: someone replays a valid signature against a
// different visitor ID.
func TestGetExternalUserReplayedSignatureDowngradesToGuest(t *testing.T) {
	ctx, _ := newIdentityContext(signedIdentityHeaders(testSecret, testChannelID, "attacker_1", time.Now()))
	ctx.Request.Header.Set("X-External-Id", "victim_999")

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, ChannelID: testChannelID})
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
	ctx, _ := newIdentityContext(signedIdentityHeaders(testSecret, testChannelID, testUserID, time.Now()))

	user, err := GetExternalUser(ctx, Secrets{ChannelID: testChannelID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceGuest {
		t.Fatalf("ExternalSource = %q, want guest when no secret is configured", user.ExternalSource)
	}
}

func TestGetExternalUserExpiredSignatureIsGuest(t *testing.T) {
	ctx, _ := newIdentityContext(signedIdentityHeaders(testSecret, testChannelID, testUserID, time.Now().Add(-25*time.Hour)))

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, IdentityMaxAge: 24 * time.Hour, ChannelID: testChannelID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceGuest {
		t.Fatalf("ExternalSource = %q, want guest for an expired signature", user.ExternalSource)
	}
}

func TestGetExternalUserRequiresAnExternalID(t *testing.T) {
	ctx, _ := newIdentityContext(map[string]string{"X-Channel-Id": testChannelID})

	if _, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, ChannelID: testChannelID}); err == nil {
		t.Fatal("expected an error when no external id is supplied")
	}
}

func TestGetExternalUserAcceptsIdentityFromQuery(t *testing.T) {
	now := time.Now()
	signature := SignExternalID(testSecret, testChannelID, testUserID, now)

	ctx, _ := newIdentityContext(nil)
	ctx.Request = httptest.NewRequest(http.MethodGet,
		"/api/customer/session_exchange?externalId="+testUserID+
			"&externalIdSignature="+signature+
			"&externalIdSignedAt="+itoa(now.Unix()), nil)

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, ChannelID: testChannelID})
	if err != nil {
		t.Fatalf("signed id from the query string must be accepted: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceUser {
		t.Fatalf("ExternalSource = %q, want %q", user.ExternalSource, enums.ExternalSourceUser)
	}
}

func TestGetExternalUserIgnoresGarbageIssuedAt(t *testing.T) {
	now := time.Now()
	ctx, _ := newIdentityContext(map[string]string{
		"X-External-Id":           testUserID,
		"X-External-Id-Signature": SignExternalID(testSecret, testChannelID, testUserID, now),
		"X-External-Id-Signed-At": "not-a-number",
	})

	user, err := GetExternalUser(ctx, Secrets{IdentityHMAC: testSecret, ChannelID: testChannelID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.ExternalSource != enums.ExternalSourceGuest {
		t.Fatalf("ExternalSource = %q, want guest for an unparsable timestamp", user.ExternalSource)
	}
}

func signedIdentityHeaders(secret, channelID, externalID string, issuedAt time.Time) map[string]string {
	return map[string]string{
		"X-External-Id":           externalID,
		"X-External-Id-Signature": SignExternalID(secret, channelID, externalID, issuedAt),
		"X-External-Id-Signed-At": itoa(issuedAt.UTC().Unix()),
		"X-External-Name":         "Ivan",
	}
}

func newIdentityContext(headers map[string]string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/customer/session_exchange", nil)
	for key, value := range headers {
		ctx.Request.Header.Set(key, value)
	}
	return ctx, recorder
}

func flipLastChar(signature string) string {
	last := signature[len(signature)-1]
	replacement := byte('0')
	if last == '0' {
		replacement = '1'
	}
	return signature[:len(signature)-1] + string(replacement)
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
