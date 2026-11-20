defmodule CouncilHubUiWeb.CouncilComponentsReceiptsTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest

  alias CouncilHubUiWeb.CouncilComponents

  defp bubble(mentions, receipts) do
    render_component(&CouncilComponents.message_bubble/1, %{
      msg: %{
        id: "uuid-rc",
        author: "alice",
        content: "ping",
        message_type: "message",
        reply_to: "",
        mentions: mentions,
        mention_receipts: receipts,
        timestamp: ~N[2026-03-29 14:00:00]
      }
    })
  end

  describe "mention chips" do
    test "seen and pending render per mentioned agent, with an absolute UTC tooltip" do
      html = bubble("claude, gemini", [{"claude", ~N[2026-03-29 14:05:00]}])

      assert html =~ "@claude"
      assert html =~ "seen "
      assert html =~ "Seen 2026-03-29 14:05:00 UTC"
      assert html =~ "@gemini"
      assert html =~ "pending"
      assert html =~ "Pending: the agent has not fetched"
    end

    test "all pending when receipts are known but empty" do
      html = bubble("claude", [])
      assert html =~ "pending"
      refute html =~ "seen "
    end

    test "unknown receipts (nil, older Go server) render plain tags, never pending" do
      html = bubble("claude", nil)
      assert html =~ "@claude"
      refute html =~ "pending"
      refute html =~ "seen "
    end
  end

  describe "agents_panel/1" do
    test "renders nothing when empty" do
      html = render_component(&CouncilComponents.agents_panel/1, %{agents: []})
      refute html =~ "Agents"
    end

    test "lists agents in the order given with last tool and relative time" do
      agents = [
        %{agent: "claude-code", last_tool: "post_to_room", last_seen_at: NaiveDateTime.utc_now()},
        %{
          agent: "gemini",
          last_tool: "read_room",
          last_seen_at: NaiveDateTime.add(NaiveDateTime.utc_now(), -7200, :second)
        }
      ]

      html = render_component(&CouncilComponents.agents_panel/1, %{agents: agents})
      assert html =~ "Agents"
      assert html =~ "post_to_room"
      assert html =~ "read_room"
      assert html =~ "just now"
      assert html =~ "2h ago"
      {i1, _} = :binary.match(html, "claude-code")
      {i2, _} = :binary.match(html, "gemini")
      assert i1 < i2
    end
  end
end
