---
type: llm
---
The sandbox has no Council Hub server connected, so the agent cannot run the tools. Pass if the answer still lays out the specific approach and tool calls it would make once connected, and the approach matches this: Says not to recreate it; explains node-local get_or_create_room shadows, peer-owned rooms and cluster_wide/list_rooms. Fail if it only says the server is not connected and tells the user to reconnect, without the specific approach, or if the approach contradicts the above.
