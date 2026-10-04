package openidentity

import (
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"agent-desk/internal/pkg/httpx/params"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/mlogclub/simple/common/strs"
)

// ExternalUser 外部访客身份（IM 客户），与站内 AuthPrincipal 区分。
type ExternalUser struct {
	ExternalSource enums.ExternalSource `json:"externalSource"`
	ExternalID     string               `json:"externalId"`
	ExternalName   string               `json:"externalName"`
}

type UserTokenClaims struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

// Secrets holds everything needed to decide how much a visitor's claimed
// identity can be trusted.
type Secrets struct {
	// UserToken is the per-channel secret used to verify a host-issued JWT.
	UserToken string
	// IdentityHMAC is the deployment-wide shared secret a host uses to sign an
	// external visitor ID. When empty, no signature is ever accepted and every
	// visitor stays anonymous.
	IdentityHMAC string
	// IdentityMaxAge bounds how old a signed identity may be. Zero means
	// DefaultIdentityMaxAge.
	IdentityMaxAge time.Duration
	// ChannelID is bound into the signed payload so a signature obtained for one
	// channel cannot be replayed on another.
	ChannelID string
}

// DefaultIdentityMaxAge is how long a signed visitor identity stays acceptable
// when identity.maxAgeMinutes is not configured.
const DefaultIdentityMaxAge = 24 * time.Hour

// identityFutureSkewTolerance absorbs modest clock differences between the host
// platform and this server. It is deliberately far smaller than the max age, so
// it widens the replay window by minutes rather than by hours.
const identityFutureSkewTolerance = 5 * time.Minute

// SignedIdentityPayload is the exact byte string a host must sign to have its
// external visitor ID treated as identified.
//
//	HMAC-SHA256(identitySecret, channelId + "\n" + externalId + "\n" + issuedAt)
//
// issuedAt is unix seconds. Lower-case hex output. Three details are deliberate:
//
//   - The channel id is included because CustomerIdentity is keyed by
//     (source, externalId) globally, so without it a signature minted for one
//     channel would assert the same identity on every other channel.
//   - The timestamp is inside the signed bytes even though it also travels as its
//     own field, so it cannot be re-dated without breaking the signature. This
//     bounds how long a leaked pair stays usable; it is not a defence against key
//     recovery, which HMAC does not permit in the first place.
//   - Nothing is encrypted. The signature passes through the visitor's browser,
//     so a cipher would hide nothing from the party we are defending against.
func SignedIdentityPayload(channelID, externalID string, issuedAt time.Time) string {
	return channelID + "\n" + externalID + "\n" + strconv.FormatInt(issuedAt.UTC().Unix(), 10)
}

// SignExternalID produces the signature a host sends alongside its external
// visitor ID and the issuedAt value it was signed over. Exported so integrators
// and tests share one definition.
func SignExternalID(secret, channelID, externalID string, issuedAt time.Time) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(SignedIdentityPayload(channelID, externalID, issuedAt)))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignExternalIDNow is the form a host calls: sign the id as of now.
func SignExternalIDNow(secret, channelID, externalID string) (signature string, issuedAt int64) {
	now := time.Now()
	return SignExternalID(secret, channelID, externalID, now), now.UTC().Unix()
}

// VerifyExternalIDSignature reports whether signature is a currently valid
// signature for externalID on this channel. A blank secret disables the feature
// rather than failing, so an unconfigured deployment keeps today's behaviour.
func VerifyExternalIDSignature(secret, channelID, externalID, signature string, issuedAtUnix int64, maxAge time.Duration, now time.Time) bool {
	if strs.IsBlank(secret) || strs.IsBlank(externalID) || strs.IsBlank(signature) || issuedAtUnix <= 0 {
		return false
	}
	if maxAge <= 0 {
		maxAge = DefaultIdentityMaxAge
	}

	issuedAt := time.Unix(issuedAtUnix, 0)
	age := now.Sub(issuedAt)
	if age > maxAge || age < -identityFutureSkewTolerance {
		return false
	}

	expected := SignExternalID(secret, channelID, externalID, issuedAt)
	// hmac.Equal is constant time, so a caller cannot recover the expected
	// signature byte by byte by timing the rejection.
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

func GetExternalUser(ctx *gin.Context, secrets Secrets) (*ExternalUser, error) {
	if userToken := getUserToken(ctx); strs.IsNotBlank(userToken) {
		claims, err := verifyUserToken(userToken, secrets.UserToken)
		if err != nil {
			return nil, err
		}
		return &ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     claims.UserID,
			ExternalName:   claims.Name,
		}, nil
	}
	return getExternalIDUser(ctx, secrets)
}

func verifyUserToken(userToken, secret string) (*UserTokenClaims, error) {
	if strs.IsBlank(userToken) {
		return nil, errorsx.UnauthorizedI18n("error.e0263")
	}
	if strs.IsBlank(secret) {
		return nil, errorsx.UnauthorizedI18n("error.e0266")
	}

	claims := &UserTokenClaims{}
	token, err := jwt.ParseWithClaims(userToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unsupported signing method")
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Alg(),
		jwt.SigningMethodHS384.Alg(),
		jwt.SigningMethodHS512.Alg(),
	}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errorsx.UnauthorizedI18n("error.e0264")
		}
		return nil, errorsx.UnauthorizedI18n("error.e0265")
	}
	if token == nil || !token.Valid {
		return nil, errorsx.UnauthorizedI18n("error.e0265")
	}

	if strs.IsBlank(claims.UserID) {
		return nil, errorsx.UnauthorizedI18n("error.e0262")
	}
	if strs.IsBlank(claims.Name) {
		return nil, errorsx.UnauthorizedI18n("error.e0261")
	}
	if claims.ExpiresAt == nil {
		return nil, errorsx.UnauthorizedI18n("error.e0264")
	}

	return claims, nil
}

func getUserToken(ctx *gin.Context) string {
	auth := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		if token := strings.TrimSpace(auth[7:]); token != "" {
			return token
		}
	}
	userToken, _ := params.Get(ctx, "userToken")
	return strings.TrimSpace(userToken)
}

// getExternalIDUser resolves the visitor from an external ID. A host that signs
// its ID with the shared identity secret is trusted and treated as identified;
// anything else, including a missing or wrong signature, is downgraded to an
// anonymous guest rather than rejected, so the plain embed keeps working.
func getExternalIDUser(ctx *gin.Context, secrets Secrets) (*ExternalUser, error) {
	externalID := getExternalID(ctx)
	if strs.IsBlank(externalID) {
		return nil, errorsx.UnauthorizedI18n("error.e0262")
	}
	externalID = strings.TrimSpace(externalID)
	externalName := getExternalName(ctx)

	if VerifyExternalIDSignature(secrets.IdentityHMAC, secrets.ChannelID, externalID, getExternalIDSignature(ctx), getExternalIDIssuedAt(ctx), secrets.IdentityMaxAge, time.Now()) {
		return &ExternalUser{
			ExternalSource: enums.ExternalSourceUser,
			ExternalID:     externalID,
			ExternalName:   externalName,
		}, nil
	}

	return &ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     externalID,
		ExternalName:   externalName,
	}, nil
}

func getExternalID(ctx *gin.Context) string {
	externalID := ctx.GetHeader("X-External-Id")
	if strs.IsBlank(externalID) {
		externalID, _ = params.Get(ctx, "externalId")
	}
	return externalID
}

func getExternalIDSignature(ctx *gin.Context) string {
	signature := ctx.GetHeader("X-External-Id-Signature")
	if strs.IsBlank(signature) {
		signature, _ = params.Get(ctx, "externalIdSignature")
	}
	return signature
}

// getExternalIDIssuedAt reads the unix-seconds timestamp the host signed. It is
// covered by the signature, so a tampered value simply fails to match.
func getExternalIDIssuedAt(ctx *gin.Context) int64 {
	raw := ctx.GetHeader("X-External-Id-Signed-At")
	if strs.IsBlank(raw) {
		raw, _ = params.Get(ctx, "externalIdSignedAt")
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	issuedAt, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return issuedAt
}

func getExternalName(ctx *gin.Context) string {
	externalName := ctx.GetHeader("X-External-Name")
	if strs.IsBlank(externalName) {
		externalName, _ = params.Get(ctx, "externalName")
	}
	if strs.IsNotBlank(externalName) {
		externalName, _ = url.QueryUnescape(externalName)
	}
	return externalName
}

func decodeExternalDisplayName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	dec, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(dec)
}
