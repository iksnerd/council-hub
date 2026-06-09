package council

import "testing"

func TestCommitBaseURL(t *testing.T) {
	cases := []struct {
		repo string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"iksnerd/council-hub", "https://github.com/iksnerd/council-hub"},
		{"iksnerd/council-hub.git", "https://github.com/iksnerd/council-hub"},
		{"https://github.com/iksnerd/council-hub", "https://github.com/iksnerd/council-hub"},
		{"https://github.com/iksnerd/council-hub.git", "https://github.com/iksnerd/council-hub"},
		{"https://github.com/iksnerd/council-hub/", "https://github.com/iksnerd/council-hub"},
		{"git@github.com:iksnerd/council-hub.git", "https://github.com/iksnerd/council-hub"},
		{"git@gitea.example.com:team/repo", "https://gitea.example.com/team/repo"},
		{"not a repo at all", ""},
	}
	for _, c := range cases {
		if got := commitBaseURL(c.repo); got != c.want {
			t.Errorf("commitBaseURL(%q) = %q, want %q", c.repo, got, c.want)
		}
	}
}

func TestResolveCommitRefsWithRepo(t *testing.T) {
	got := ResolveCommitRefs("Shipped {sha:89cfaf1abc} — hardened poller", "iksnerd/council-hub")
	want := "Shipped [`89cfaf1`](https://github.com/iksnerd/council-hub/commit/89cfaf1abc) — hardened poller"
	if got != want {
		t.Errorf("resolveCommitRefs = %q, want %q", got, want)
	}
}

func TestResolveCommitRefsShortHashNotTruncated(t *testing.T) {
	// A 7-char hash is displayed verbatim; the link still uses the full token.
	got := ResolveCommitRefs("see {sha:abc1234}", "owner/repo")
	want := "see [`abc1234`](https://github.com/owner/repo/commit/abc1234)"
	if got != want {
		t.Errorf("resolveCommitRefs = %q, want %q", got, want)
	}
}

func TestResolveCommitRefsNoRepoFallsBackToCodeSpan(t *testing.T) {
	got := ResolveCommitRefs("fixed in {sha:deadbeef}", "")
	want := "fixed in `deadbee`"
	if got != want {
		t.Errorf("resolveCommitRefs = %q, want %q", got, want)
	}
}

func TestResolveCommitRefsMultipleTokens(t *testing.T) {
	got := ResolveCommitRefs("{sha:aaaaaaa} then {sha:bbbbbbb}", "owner/repo")
	want := "[`aaaaaaa`](https://github.com/owner/repo/commit/aaaaaaa) then [`bbbbbbb`](https://github.com/owner/repo/commit/bbbbbbb)"
	if got != want {
		t.Errorf("resolveCommitRefs = %q, want %q", got, want)
	}
}

func TestResolveCommitRefsIgnoresNonMatches(t *testing.T) {
	// Too short, non-hex, and malformed tokens are left untouched.
	in := "{sha:short} {sha:nothex!} {sha:} plain text"
	if got := ResolveCommitRefs(in, "owner/repo"); got != in {
		t.Errorf("resolveCommitRefs mangled non-matching input: %q", got)
	}
}

func TestResolveCommitRefsNoTokenIsIdentity(t *testing.T) {
	in := "a message with no commit refs at all"
	if got := ResolveCommitRefs(in, "owner/repo"); got != in {
		t.Errorf("resolveCommitRefs changed token-free content: %q", got)
	}
}
