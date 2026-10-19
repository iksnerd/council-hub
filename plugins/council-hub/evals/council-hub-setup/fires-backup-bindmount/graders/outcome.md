---
type: llm
---
The sandbox has no Council Hub server connected, so the agent cannot run the tools. Pass if the answer still lays out the specific approach and tool calls it would make once connected, and the approach matches this: Warns against copying or opening the db from the host while the container runs (bind mount WAL hazard) and gives docker exec council-hub council-hub -backup, preferring a named volume. Fail if it only says the server is not connected and tells the user to reconnect, without the specific approach, or if the approach contradicts the above.
