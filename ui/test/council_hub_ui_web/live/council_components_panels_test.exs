defmodule CouncilHubUiWeb.CouncilComponentsPanelsTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest

  alias CouncilHubUiWeb.CouncilComponents

  describe "mentions_panel/1" do
    test "renders nothing when mentions list is empty" do
      html = render_component(&CouncilComponents.mentions_panel/1, %{mentions: []})
      refute html =~ "Mentions"
    end

    test "renders mention entries when present" do
      mentions = [
        %{
          id: "m1",
          room_id: "some-room",
          author: "gemini",
          content: "hey claude check this",
          timestamp: ~N[2026-04-08 10:00:00]
        }
      ]

      html = render_component(&CouncilComponents.mentions_panel/1, %{mentions: mentions})
      assert html =~ "Mentions"
      assert html =~ "some-room"
      assert html =~ "gemini"
      assert html =~ "hey claude check this"
    end

    test "links to the source room" do
      mentions = [
        %{
          id: "m2",
          room_id: "target-room",
          author: "warp",
          content: "yo",
          timestamp: ~N[2026-04-08 10:00:00]
        }
      ]

      html = render_component(&CouncilComponents.mentions_panel/1, %{mentions: mentions})
      assert html =~ "/rooms/target-room"
    end
  end

  describe "archive_list/1" do
    test "renders nothing when archives list is empty" do
      html = render_component(&CouncilComponents.archive_list/1, %{archives: []})
      refute html =~ "Archives"
    end

    test "renders archive entries when present" do
      archives = [
        %{"room_id" => "old-room", "archived_at" => "2026-04-01T10:00:00Z", "size" => 1024}
      ]

      html = render_component(&CouncilComponents.archive_list/1, %{archives: archives})
      assert html =~ "Archives"
      assert html =~ "old-room"
      assert html =~ "2026-04-01"
    end

    test "archive buttons have view_archive phx-click" do
      archives = [
        %{"room_id" => "click-room", "archived_at" => "2026-04-01T10:00:00Z", "size" => 512}
      ]

      html = render_component(&CouncilComponents.archive_list/1, %{archives: archives})
      assert html =~ ~s(phx-click="view_archive")
      assert html =~ ~s(phx-value-room-id="click-room")
    end
  end

  describe "archive_modal/1" do
    test "renders nothing when active_archive is nil" do
      html = render_component(&CouncilComponents.archive_modal/1, %{active_archive: nil})
      refute html =~ "archive"
    end

    test "renders modal with room_id and content when active" do
      active = %{room_id: "my-room", content: "## Summary\n\nsome content"}
      html = render_component(&CouncilComponents.archive_modal/1, %{active_archive: active})
      assert html =~ "my-room"
      assert html =~ "Summary"
      assert html =~ ~s(phx-click="close_archive")
    end
  end
end
