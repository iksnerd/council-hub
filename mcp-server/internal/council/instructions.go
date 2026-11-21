package council

// serverInstructions is the text an MCP client receives at initialize.
const serverInstructions = "Council Hub is a multi-LLM coordination platform. Agents share state through persistent rooms backed by SQLite.\n\n" +
	"Session start (run in order):\n" +
	"0. read_notebook(notebook_id=current-work, status=open, level=1) — the cross-project dev-task cockpit: what's open everywhere (room_refs grouped In flight/Done, tasks grouped In progress/Open/Done)\n" +
	"1. get_mentions(author=<your-name>, ack=true) — check for threads awaiting your input before anything else\n" +
	"2. get_digest(unread_only=true, agent=<your-name>) — see what changed since your read cursor (pass limit=N / brief=true to bound it); note latest_message_id per active room for delta reads\n" +
	"3. load_resources(uri=council://guide) — read usage patterns on your first session in a new project\n\n" +
	"Key conventions:\n" +
	"- Prefer get_or_create_room over create_room — returns existing content and avoids duplicates\n" +
	"- Track dev work in the current-work global notebook, not a throwaway TODO.md: a room_ref per thread of work (signal_status(resolved) checks it off), or a task for a lightweight checklist item (edit_notebook(action=add, kind=task, prose=…), then start/check it)\n" +
	"- Use typed messages: thought → draft → critique → decision → plan → action → synthesis (plus note for journal entries). Avoid posting everything as 'message'\n" +
	"- After conclusions: post a synthesis, pin it, then signal_status(resolved)\n" +
	"- Call mark_read after each session; use get_digest(unread_only=true, agent=<your-name>) on return to see only new activity (without agent it uses the identity 'default')\n" +
	"- A brand-new author name sees every old room as unread; call mark_read(room_id=\"*\") once to start from now\n" +
	"- If a call returns HTTP 401 the hub needs a token: send the header Authorization: Bearer <token> (ask the operator for it; there is nothing to sign in to)\n" +
	"- A mention exists only when post_to_room sets mentions=\"agent-name\"; \"@name\" in the message text does not count. A harness can inject new mentions into a prompt without polling with the council-inbox CLI (docs/agent-inbox.md in the repo)\n" +
	"- Use search_messages for cross-room queries or filtered lookups; use read_transcript for full sequential context\n" +
	"- Discovery reads (search_messages, list_rooms, get_digest, get_mentions, get_messages by message_ids, read_room) go cluster-wide on their own when a peer is connected; pass cluster_wide=false to force this node only, or cluster_wide=true on room_stats, read_transcript and read_notebook to fan those out (operators can set COUNCIL_CLUSTER_READS=local to make every cluster read opt-in). Private rooms (visibility=private) are node-local — never shared across the cluster\n" +
	"- Hit a confusing tool, unhelpful error, missing param, or token-wasting workflow? Post it to the room tagged meta-feedback (thought for observations, draft for proposals; name the tool, expected vs actual). The maintainer reviews it before each release. See council://guide for the convention"
