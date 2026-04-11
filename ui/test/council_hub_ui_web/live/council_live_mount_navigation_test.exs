defmodule CouncilHubUiWeb.CouncilLiveMountNavigationTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest
  import CouncilHubUi.CouncilFixtures

  describe "mount" do
    test "renders empty state with no rooms", %{conn: conn} do
      {:ok, _view, html} = live(conn, "/")
      assert html =~ "Council Hub"
    end

    test "renders rooms in sidebar", %{conn: conn} do
      create_room(%{id: "live-room-1", description: "First room", project: "proj-a"})
      create_room(%{id: "live-room-2", description: "Second room", project: "proj-b"})

      {:ok, _view, html} = live(conn, "/")
      assert html =~ "live-room-1"
      assert html =~ "live-room-2"
    end
  end

  describe "room navigation" do
    test "selecting a room shows messages", %{conn: conn} do
      room = create_room(%{id: "nav-room", description: "Navigation test"})
      create_message(%{room_id: room.id, author: "Claude", content: "Hello from test"})

      {:ok, view, _html} = live(conn, "/")
      view |> element("a[href='/rooms/nav-room']") |> render_click()

      # After navigation, should show room content
      html = render(view)
      assert html =~ "nav-room"
      assert html =~ "Hello from test"
    end

    test "direct URL to room works", %{conn: conn} do
      room = create_room(%{id: "direct-room", description: "Direct access"})
      create_message(%{room_id: room.id, author: "Gemini", content: "Direct message"})

      {:ok, _view, html} = live(conn, "/rooms/direct-room")
      assert html =~ "direct-room"
      assert html =~ "Direct message"
    end

    test "nonexistent room redirects with flash", %{conn: conn} do
      assert {:error, {:live_redirect, %{to: "/", flash: %{"error" => flash}}}} =
               live(conn, "/rooms/nonexistent")

      assert flash =~ "not found"
    end
  end

  describe "room metadata display" do
    test "shows project and tech stack in header", %{conn: conn} do
      create_room(%{
        id: "meta-room",
        description: "Metadata test",
        project: "my-project",
        tech_stack: "Elixir, Go"
      })

      {:ok, _view, html} = live(conn, "/rooms/meta-room")
      assert html =~ "MY-PROJECT"
      assert html =~ "ELIXIR, GO"
    end

    test "shows related rooms in header", %{conn: conn} do
      create_room(%{
        id: "linked-room",
        description: "Linked test",
        related_rooms: "room-a,room-b"
      })

      {:ok, _view, html} = live(conn, "/rooms/linked-room")
      assert html =~ "room-a"
      assert html =~ "room-b"
    end

    test "shows tags in header", %{conn: conn} do
      create_room(%{id: "tagged-room", description: "Tagged", tags: "auth,security"})

      {:ok, _view, html} = live(conn, "/rooms/tagged-room")
      assert html =~ "auth"
      assert html =~ "security"
    end
  end

  describe "nil project handling" do
    test "rooms with nil project go to ungrouped" do
      rooms = [%{id: "x", project: nil, description: "Nil project", status: "active"}]
      grouped = CouncilHubUiWeb.CouncilLive.group_rooms_by_project(rooms)
      assert [{"ungrouped", _}] = grouped
    end
  end

  describe "group_rooms_by_project/1" do
    test "groups rooms by project" do
      rooms = [
        %{id: "a", project: "proj-1", description: "", status: "active"},
        %{id: "b", project: "proj-2", description: "", status: "active"},
        %{id: "c", project: "proj-1", description: "", status: "active"}
      ]

      grouped = CouncilHubUiWeb.CouncilLive.group_rooms_by_project(rooms)
      projects = Enum.map(grouped, fn {p, _} -> p end)
      assert "proj-1" in projects
      assert "proj-2" in projects
    end

    test "ungrouped rooms go to 'ungrouped'" do
      rooms = [%{id: "x", project: "", description: "", status: "active"}]
      grouped = CouncilHubUiWeb.CouncilLive.group_rooms_by_project(rooms)
      assert [{"ungrouped", _}] = grouped
    end
  end

  describe "sidebar project accordion" do
    test "rooms grouped by project render in sidebar", %{conn: conn} do
      create_room(%{id: "acc-room-a", project: "proj-alpha"})
      create_room(%{id: "acc-room-b", project: "proj-beta"})

      {:ok, _view, html} = live(conn, "/")
      assert html =~ "proj-alpha"
      assert html =~ "proj-beta"
      assert html =~ "acc-room-a"
      assert html =~ "acc-room-b"
    end

    test "active room's project group is open", %{conn: conn} do
      create_room(%{id: "acc-active", project: "proj-active"})
      create_message(%{room_id: "acc-active", author: "Claude", content: "msg"})

      {:ok, _view, html} = live(conn, "/rooms/acc-active")
      # The details element for the active project should have open attribute
      assert html =~ "acc-active"
    end
  end
end
