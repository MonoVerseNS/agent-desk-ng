// Package loginaudience decides which sign-in surface a login attempt came
// from, based on the post-login destination the client asked for.
package loginaudience

import "strings"

type Audience int

const (
	// AudienceStaff is the agent/admin dashboard.
	AudienceStaff Audience = iota
	// AudiencePortal is the visitor-facing support portal.
	AudiencePortal
)

const portalRoot = "/support"

// Resolve classifies the `next` destination of a login start.
//
// Enterprise SSO (OIDC, WeCom) provisions a User with
// enums.UserTypeEmployee and a staff role, and never links that user to a
// Customer. So a visitor signing in through the portal over SSO would receive a
// staff session with no history and no customer identity behind it - an
// escalation the portal has no use for, since portal visitors authenticate with
// a password or not at all.
//
// An absent destination means staff: that is the historical behaviour and the
// only place those transports are meant to be started from.
func Resolve(next string) Audience {
	path := strings.TrimSpace(next)
	if i := strings.IndexAny(path, "?#"); i >= 0 {
		path = path[:i]
	}
	// A trailing slash must not turn /support/ into a different answer than
	// /support, so compare on a slash-trimmed path.
	path = strings.TrimRight(path, "/")

	if path == portalRoot || strings.HasPrefix(path, portalRoot+"/") {
		return AudiencePortal
	}
	return AudienceStaff
}

func (a Audience) IsPortal() bool {
	return a == AudiencePortal
}

func (a Audience) String() string {
	if a == AudiencePortal {
		return "portal"
	}
	return "staff"
}
