package council

import (
	"regexp"
	"strings"
)

// MIRROR: this resolver is duplicated in the Phoenix UI
// (ui/lib/council_hub_ui_web/live/council_helpers.ex — resolve_commit_refs/2,
// repo_url/1) because Go and the BEAM render the same content independently and
// can't share code. The two must stay byte-for-byte identical (regexes,
// short-SHA rule, URL normalization). Their paired tests are load-bearing — if
// you change one side, change the other and update both test files.

// commitRefRe matches {sha:<hash>} tokens where hash is 7–40 hex characters.
var workItemRefRe = regexp.MustCompile(`\{wi:([A-Za-z0-9][A-Za-z0-9._-]*)\}`)

var commitRefRe = regexp.MustCompile(`\{sha:([0-9a-fA-F]{7,40})\}`)

// ownerRepoRe matches a bare GitHub-style owner/repo slug.
var ownerRepoRe = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

// scpRemoteRe matches scp-style git remotes like git@github.com:owner/repo.
var scpRemoteRe = regexp.MustCompile(`^[\w.-]+@([\w.-]+):(.+)$`)

// commitBaseURL converts a room's repo reference into a base URL whose
// "/commit/<sha>" path points at a single commit. Returns "" if repo is empty
// or unrecognised, in which case the caller renders the SHA without a link.
//
// Accepted forms (a trailing ".git" and "/" are stripped first):
//
//	owner/repo                    -> https://github.com/owner/repo
//	https://host/owner/repo       -> https://host/owner/repo
//	git@host:owner/repo           -> https://host/owner/repo
//
// The "/commit/<sha>" path is GitHub/Gitea-style; GitLab (which uses
// "/-/commit/") is out of scope for this resolver.
func commitBaseURL(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimSuffix(repo, "/")
	repo = strings.TrimSuffix(repo, ".git")
	if repo == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(repo, "http://"), strings.HasPrefix(repo, "https://"):
		return repo
	case ownerRepoRe.MatchString(repo):
		return "https://github.com/" + repo
	default:
		if m := scpRemoteRe.FindStringSubmatch(repo); m != nil {
			return "https://" + m[1] + "/" + m[2]
		}
	}
	return ""
}

// ResolveCommitRefs replaces {sha:<hash>} tokens in content with a short-SHA
// markdown commit link when repo resolves to a known host, or a bare `short`
// code span when it doesn't. It is render-time only, read-only, and makes no
// network calls — the link target is constructed purely from the SHA and repo.
func ResolveCommitRefs(content, repo string) string {
	if !strings.Contains(content, "{sha:") {
		return content
	}
	base := commitBaseURL(repo)
	return commitRefRe.ReplaceAllStringFunc(content, func(tok string) string {
		full := commitRefRe.FindStringSubmatch(tok)[1]
		short := full
		if len(short) > 7 {
			short = short[:7]
		}
		if base == "" {
			return "`" + short + "`"
		}
		return "[`" + short + "`](" + base + "/commit/" + full + ")"
	})
}

// ResolveWorkItemRefs replaces {wi:<id>} tokens with a markdown link built from
// the room's work_items URL template ({id} is substituted), or a bare `id` code
// span when the room has no template. Render-time only, no network calls.
func ResolveWorkItemRefs(content, template string) string {
	if !strings.Contains(content, "{wi:") {
		return content
	}
	template = strings.TrimSpace(template)
	return workItemRefRe.ReplaceAllStringFunc(content, func(tok string) string {
		id := workItemRefRe.FindStringSubmatch(tok)[1]
		if template == "" || !strings.Contains(template, "{id}") {
			return "`" + id + "`"
		}
		return "[" + id + "](" + strings.ReplaceAll(template, "{id}", id) + ")"
	})
}

// ResolveRefs applies both {sha:...} and {wi:...} resolution; the single entry
// point for every render path that has a room's repo and work_items in hand.
func ResolveRefs(content, repo, workItems string) string {
	return ResolveWorkItemRefs(ResolveCommitRefs(content, repo), workItems)
}
