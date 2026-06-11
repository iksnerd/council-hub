defmodule CouncilHubUiWeb.NotebookLiveTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

  defp seed do
    room_a = create_room(%{id: "nbl-room-a", project: "nbl-proj", repo: "alice/widgets"})
    room_b = create_room(%{id: "nbl-room-b", project: "nbl-proj"})

    create_message(%{
      room_id: room_a.id,
      author: "claude",
      content: "decided on SQLite {sha:abc1234}",
      message_type: "decision"
    })

    create_message(%{room_id: room_a.id, content: "plain chatter"})

    create_message(%{
      room_id: room_b.id,
      author: "gemini",
      content: "parser shipped",
      message_type: "action"
    })
  end

  describe "notebook page" do
    test "renders the timeline for the default project", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/notebook")

      assert html =~ "Notebook"
      assert html =~ "decided on SQLite"
      assert html =~ "parser shipped"
      # default types exclude plain messages
      refute html =~ "plain chatter"
      # {sha:...} resolved against the owning room's repo
      assert html =~ "https://github.com/alice/widgets/commit/abc1234"
    end

    test "shows the empty state when there are no projects", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/notebook")
      assert html =~ "No projects yet"
    end

    test "respects project and types query params", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/notebook?project=nbl-proj&types=action")

      refute html =~ "decided on SQLite"
      assert html =~ "parser shipped"
    end

    test "toggle_type patches the URL and reloads entries", %{conn: conn} do
      seed()

      {:ok, view, _html} = live(conn, "/notebook?project=nbl-proj")

      html =
        view
        |> element("button[phx-value-type=decision]")
        |> render_click()

      # decision toggled off — only the action remains
      assert_patch(view)
      refute html =~ "decided on SQLite"
      assert html =~ "parser shipped"
    end

    test "entries link to their room", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/notebook?project=nbl-proj")
      assert html =~ "/rooms/nbl-room-a"
    end
  end

  describe "outline mode" do
    defp seed_outline do
      seed()
      create_notebook(%{id: "nbl-outline", project: "nbl-proj", title: "Release Notes"})

      msg =
        create_message(%{
          room_id: "nbl-room-a",
          author: "claude",
          content: "outline-worthy decision {sha:def5678}",
          message_type: "decision"
        })

      create_notebook_entry(%{
        notebook_id: "nbl-outline",
        position: 1,
        kind: "prose",
        prose: "## Shipped"
      })

      create_notebook_entry(%{
        notebook_id: "nbl-outline",
        position: 2,
        kind: "ref",
        ref_id: msg.id
      })

      create_notebook_entry(%{
        notebook_id: "nbl-outline",
        position: 3,
        kind: "ref",
        ref_id: "ghost-id"
      })
    end

    test "timeline lists curated notebooks as links", %{conn: conn} do
      seed_outline()

      {:ok, _view, html} = live(conn, "/notebook?project=nbl-proj")
      assert html =~ "Curated notebooks"
      assert html =~ "nbl-outline"
    end

    test "renders the outline with transcluded refs and dangling-ref warning", %{conn: conn} do
      seed_outline()

      {:ok, _view, html} = live(conn, "/notebook?project=nbl-proj&notebook=nbl-outline")

      assert html =~ "Release Notes"
      assert html =~ "Shipped"
      # transcluded ref resolves content and commit link against the room repo
      assert html =~ "outline-worthy decision"
      assert html =~ "https://github.com/alice/widgets/commit/def5678"
      # dangling ref warns instead of crashing
      assert html =~ "not found"
      # back link to the timeline
      assert html =~ "← timeline"
    end

    test "unknown notebook shows a flash error", %{conn: conn} do
      seed()

      {:ok, _view, html} = live(conn, "/notebook?project=nbl-proj&notebook=ghost")
      assert html =~ "not found"
    end
  end
end
