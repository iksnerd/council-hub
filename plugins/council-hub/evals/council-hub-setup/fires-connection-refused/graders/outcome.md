---
type: llm
---
The sandbox has no Council Hub server connected, so the agent cannot run the tools. Pass if the answer still lays out the specific approach and tool calls it would make once connected, and the approach matches this: Tells the user to run docker port council-hub (and check /health, reconnect with /mcp), explaining that healthy does not mean ports are published. Fail if it only says the server is not connected and tells the user to reconnect, without the specific approach, or if the approach contradicts the above.
