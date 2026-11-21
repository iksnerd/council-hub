package council

import (
	"strings"
	"testing"
)

func TestServerInstructionsContent(t *testing.T) {
	for _, want := range []string{
		"read_notebook(notebook_id=current-work, status=open, level=1)",
		"get_digest(unread_only=true, agent=<your-name>)",
		`mark_read(room_id="*")`,
		"HTTP 401",
		"Authorization: Bearer <token>",
		"council-inbox",
		`A mention exists only when post_to_room sets mentions="agent-name"; "@name" in the message text does not count.`,
		"docs/agent-inbox.md",
		"Private rooms (visibility=private) are node-local",
		"cluster_wide=false",
	} {
		if !strings.Contains(serverInstructions, want) {
			t.Errorf("instructions missing %q", want)
		}
	}
}
