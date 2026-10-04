package loginaudience

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		next string
		want Audience
	}{
		{name: "empty destination is staff", next: "", want: AudienceStaff},
		{name: "dashboard is staff", next: "/dashboard", want: AudienceStaff},
		{name: "dashboard deep link is staff", next: "/dashboard/users", want: AudienceStaff},
		{name: "portal root is portal", next: "/support", want: AudiencePortal},
		{name: "portal deep link is portal", next: "/support/chat", want: AudiencePortal},
		{name: "portal trailing slash is portal", next: "/support/", want: AudiencePortal},
		{name: "portal with query is portal", next: "/support/community?page=2", want: AudiencePortal},
		{name: "portal with hash is portal", next: "/support/profile#top", want: AudiencePortal},
		{name: "padded value still resolves", next: "  /support/chat  ", want: AudiencePortal},
		// A dashboard path that merely mentions support must not be mistaken for
		// the portal, or staff SSO would break.
		{name: "dashboard path mentioning support is staff", next: "/dashboard/support-tools", want: AudienceStaff},
		{name: "support prefix without a boundary is staff", next: "/supportfoo", want: AudienceStaff},
		{name: "absolute url is staff", next: "https://evil.example/support", want: AudienceStaff},
		{name: "protocol relative is staff", next: "//evil.example/support", want: AudienceStaff},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.next); got != tt.want {
				t.Fatalf("Resolve(%q) = %s, want %s", tt.next, got, tt.want)
			}
		})
	}
}

func TestIsPortal(t *testing.T) {
	if !AudiencePortal.IsPortal() {
		t.Fatal("AudiencePortal.IsPortal() = false")
	}
	if AudienceStaff.IsPortal() {
		t.Fatal("AudienceStaff.IsPortal() = true")
	}
}

func TestString(t *testing.T) {
	if AudienceStaff.String() != "staff" {
		t.Fatalf("staff label = %q", AudienceStaff.String())
	}
	if AudiencePortal.String() != "portal" {
		t.Fatalf("portal label = %q", AudiencePortal.String())
	}
}
