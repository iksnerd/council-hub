defmodule CouncilHubUiWeb.CouncilComponentsRoomTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest

  alias CouncilHubUiWeb.CouncilComponents

  describe "room_card" do
    test "renders room id and status" do
      assigns = %{
        room: %{
          id: "test-room",
          status: "active",
          description: "A test room",
          tags: "tag1,tag2",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 5
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "test-room"
      assert html =~ "active"
      assert html =~ "5"
    end

    test "shows stale health badge when tagged stale" do
      assigns = %{
        room: %{
          id: "stale-room",
          status: "active",
          description: "",
          tags: "stale",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "stale"
      assert html =~ "red"
    end

    test "shows needs-synthesis health badge" do
      assigns = %{
        room: %{
          id: "synth-room",
          status: "active",
          description: "",
          tags: "needs-synthesis",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "border-l-amber-500"
    end

    test "no health badges for healthy room" do
      assigns = %{
        room: %{
          id: "healthy-room",
          status: "active",
          description: "",
          tags: "auth,api",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      refute html =~ "border-l-amber-500"
      refute html =~ ~r/bg-red-500.*stale/
    end

    test "no cursor shown (latest_id removed from card)" do
      assigns = %{
        room: %{
          id: "cursor-room",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0,
        latest_id: "019d0000-0000-7000-8000-abcdef123456"
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      refute html =~ "cursor:"
    end

    test "no cursor shown when latest_id is nil" do
      assigns = %{
        room: %{
          id: "no-cursor-room",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0,
        latest_id: nil
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      refute html =~ "cursor:"
    end

    test "renders active styling" do
      assigns = %{
        room: %{
          id: "active-card",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: true,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "sky"
    end

    test "renders paused status" do
      assigns = %{
        room: %{
          id: "paused-card",
          status: "paused",
          description: "Paused room",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "paused"
    end

    test "renders source_node badge for remote room" do
      assigns = %{
        room: %{
          id: "remote-room",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0,
        source_node: "council_hub@council_hub"
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "council_hub"
    end

    test "no source_node badge when source_node is nil" do
      assigns = %{
        room: %{
          id: "local-room",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0,
        source_node: nil
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      refute html =~ "bg-blue-500/10"
    end

    test "renders type breakdown when decisions and actions present" do
      assigns = %{
        room: %{
          id: "type-count-room",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 5,
        type_counts: %{"decision" => 3, "action" => 2}
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "D:3"
      assert html =~ "A:2"
    end

    test "omits type breakdown when no decisions or actions" do
      assigns = %{
        room: %{
          id: "no-types-room",
          status: "active",
          description: "",
          tags: "",
          updated_at: ~N[2026-03-29 14:00:00]
        },
        active: false,
        count: 0,
        type_counts: %{}
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      refute html =~ "0d 0a"
    end
  end

  describe "room_header" do
    test "renders room metadata" do
      assigns = %{
        room: %{
          id: "header-room",
          status: "active",
          description: "Header test",
          project: "my-proj",
          tech_stack: "Elixir, Go",
          tags: "tag1,tag2",
          system_prompt: "Be helpful",
          related_rooms: "room-a,room-b",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 10,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ "header-room"
      assert html =~ "MY-PROJ"
      assert html =~ "ELIXIR, GO"
      assert html =~ "tag1"
      assert html =~ "room-a"
      assert html =~ "room-b"
      assert html =~ "10 msgs"
    end

    test "renders related rooms as navigable links" do
      assigns = %{
        room: %{
          id: "linked-room",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "",
          related_rooms: "room-a,room-b",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ ~r/href="\/rooms\/room-a"/
      assert html =~ ~r/href="\/rooms\/room-b"/
    end

    test "shows system prompt when toggled" do
      assigns = %{
        room: %{
          id: "prompt-header",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "Secret instructions",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: true
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ "Secret instructions"
    end

    test "hides system prompt by default" do
      assigns = %{
        room: %{
          id: "hidden-prompt",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "Hidden stuff",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      refute html =~ "Hidden stuff"
    end
  end

  describe "v0.15.0 features" do
    test "room_header renders updated_at RelativeTime hook when present" do
      assigns = %{
        room: %{
          id: "updated-at-room",
          status: "active",
          description: "Test",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00],
          updated_at: ~N[2026-04-07 10:00:00]
        },
        count: 3,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ "header-updated-updated-at-room"
      assert html =~ "RelativeTime"
      assert html =~ "2026-04-07"
    end

    test "room_header omits updated_at div when not present" do
      assigns = %{
        room: %{
          id: "no-updated-at",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      refute html =~ "header-updated-no-updated-at"
    end

    test "room_header shows archive button for resolved rooms" do
      assigns = %{
        room: %{
          id: "resolved-hdr",
          status: "resolved",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ "phx-click=\"archive_room\""
    end

    test "room_header hides archive button for active rooms" do
      assigns = %{
        room: %{
          id: "active-hdr",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      refute html =~ "phx-click=\"archive_room\""
    end

    test "room_header always shows lint button" do
      assigns = %{
        room: %{
          id: "lint-hdr",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "",
          system_prompt: "",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ "phx-click=\"check_room_health\""
    end

    test "room_header shows tag editor form when editing_tags is true" do
      assigns = %{
        room: %{
          id: "tag-edit-hdr",
          status: "active",
          description: "",
          project: "",
          tech_stack: "",
          tags: "go,elixir",
          system_prompt: "",
          related_rooms: "",
          created_at: ~N[2026-03-29 14:00:00]
        },
        count: 0,
        show_system_prompt: false,
        editing_tags: true,
        tag_input: "go,elixir"
      }

      html = render_component(&CouncilComponents.room_header/1, assigns)
      assert html =~ ~s(name="tags")
      assert html =~ "save"
      assert html =~ "cancel"
    end
  end
end
