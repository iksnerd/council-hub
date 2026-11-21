defmodule CouncilHubUiWeb.AgentsPanelPresenceTest do
  use CouncilHubUiWeb.ConnCase, async: false

  import Phoenix.LiveViewTest
  alias Phoenix.ChannelTest, as: CT

  alias CouncilHubUi.AgentPresence
  alias CouncilHubUiWeb.{CouncilComponents, PresenceChannel, UserSocket}

  defp seen(agent, tool, ago \\ 60) do
    %{
      agent: agent,
      last_tool: tool,
      last_seen_at: NaiveDateTime.add(NaiveDateTime.utc_now(), -ago, :second)
    }
  end

  defp live_row(agent, attrs \\ %{}) do
    Map.merge(%{agent: agent, count: 1, clients: [], rooms: [], nodes: ["n1@10.0.0.5"]}, attrs)
  end

  defp panel(live, seen) do
    render_component(&CouncilComponents.agents_panel/1, %{agents: AgentPresence.merge(live, seen)})
  end

  describe "merge/2 and the panel" do
    test "live agents come first, then last-seen rows in their given order" do
      rows =
        AgentPresence.merge(
          [live_row("amy"), live_row("zed")],
          [seen("recent", "read_room", 10), seen("old", "post_to_room", 7200)]
        )

      assert ["amy", "zed", "recent", "old"] = Enum.map(rows, & &1.agent)
      assert [true, true, false, false] = Enum.map(rows, & &1.live)
    end

    test "an agent both live and seen gets one row" do
      assert [%{agent: "amy", live: true, last_tool: "read_room"}] =
               AgentPresence.merge([live_row("amy")], [seen("amy", "read_room")])
    end

    test "renders live dot, count, client, rooms and last-seen rows" do
      html =
        panel(
          [
            live_row("amy", %{
              count: 2,
              clients: ["codex-cli", "council-inbox"],
              rooms: ["r1", "r2"]
            })
          ],
          [seen("bob", "read_room", 7200)]
        )

      assert html =~ "bg-green-500"
      assert html =~ "live x2"
      assert html =~ "codex-cli, council-inbox"
      assert html =~ "watching r1, r2"
      assert html =~ "not verified"
      assert html =~ "last seen 2h ago"
      assert html =~ "read_room"
      {i1, _} = :binary.match(html, "amy")
      {i2, _} = :binary.match(html, "bob")
      assert i1 < i2
    end

    test "a single live connection shows no count" do
      html = panel([live_row("amy")], [])
      assert html =~ "live"
      refute html =~ "live x"
    end

    test "the node shows only when more than one node reports" do
      refute panel([live_row("amy")], []) =~ "n1@10.0.0.5"

      html = panel([live_row("amy"), live_row("bob", %{nodes: ["n2@10.0.0.6"]})], [])
      assert html =~ "n1@10.0.0.5"
      assert html =~ "n2@10.0.0.6"
    end
  end

  test "the dashboard updates on presence_diff; two connections of one agent are one row", %{
    conn: conn
  } do
    {:ok, view, _html} = live(conn, "/")
    refute render(view) =~ "lv-agent"

    for client <- ["codex-cli", "gemini-cli"] do
      {:ok, socket} = CT.__connect__(@endpoint, UserSocket, %{}, [])

      {:ok, _, _} =
        CT.subscribe_and_join(socket, PresenceChannel, "presence:agents", %{
          "agent" => "lv-agent",
          "client" => client,
          "rooms" => ["lv-room"]
        })
    end

    html = wait_render(view, "live x2")
    assert html =~ "watching lv-room"
    assert length(Regex.scan(~r/data-agent="lv-agent"/, html)) == 1
  end

  defp wait_render(view, needle, tries \\ 50) do
    html = render(view)

    cond do
      html =~ needle ->
        html

      tries == 0 ->
        flunk("never rendered #{needle}")

      true ->
        Process.sleep(20)
        wait_render(view, needle, tries - 1)
    end
  end
end
