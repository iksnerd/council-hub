package tailnet

import "testing"

func TestAllowed(t *testing.T) {
	user := Identity{LoginName: "me@example.com"}
	other := Identity{LoginName: "you@example.com"}
	tagged := Identity{Tagged: true}
	nobody := Identity{}

	tests := []struct {
		name  string
		mode  Access
		owner string
		id    Identity
		want  bool
	}{
		{"owner admits owner", AccessOwner, "me@example.com", user, true},
		{"owner match ignores case", AccessOwner, "Me@Example.com", user, true},
		{"owner refuses another user", AccessOwner, "me@example.com", other, false},
		{"owner refuses tagged device", AccessOwner, "me@example.com", tagged, false},
		{"owner refuses unidentified", AccessOwner, "me@example.com", nobody, false},
		{"owner mode with unknown owner admits nobody", AccessOwner, "", user, false},
		{"owner mode with unknown owner refuses empty login", AccessOwner, "", nobody, false},
		{"members admits another user", AccessMembers, "me@example.com", other, true},
		{"members admits tagged device", AccessMembers, "me@example.com", tagged, true},
		{"members refuses unidentified", AccessMembers, "me@example.com", nobody, false},
		{"unknown mode denies", Access("everyone"), "me@example.com", user, false},
		{"empty mode denies", Access(""), "me@example.com", user, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Allowed(tc.mode, tc.owner, tc.id); got != tc.want {
				t.Fatalf("Allowed(%q, %q, %+v) = %v, want %v", tc.mode, tc.owner, tc.id, got, tc.want)
			}
		})
	}
}

func TestAccessValid(t *testing.T) {
	if !AccessOwner.Valid() || !AccessMembers.Valid() {
		t.Fatal("owner and members must be valid")
	}
	if Access("").Valid() || Access("all").Valid() {
		t.Fatal("empty and unknown modes must be invalid")
	}
}
