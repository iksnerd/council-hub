defmodule CouncilHubUiWeb.CouncilComponentsTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest

  # Component rendering is tested end-to-end via LiveView integration tests
  # in council_live_test.exs. These tests verify specific component behavior
  # using render_component/2 from Phoenix.LiveViewTest.

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
      assert html =~ "tag1"
      assert html =~ "5"
    end

    test "shows stale health badge when tagged stale" do
      assigns = %{
        room: %{id: "stale-room", status: "active", description: "", tags: "stale",
                updated_at: ~N[2026-03-29 14:00:00]},
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "stale"
      assert html =~ "red"
    end

    test "shows needs-synthesis health badge" do
      assigns = %{
        room: %{id: "synth-room", status: "active", description: "", tags: "needs-synthesis",
                updated_at: ~N[2026-03-29 14:00:00]},
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "needs synthesis"
      assert html =~ "yellow"
    end

    test "no health badges for healthy room" do
      assigns = %{
        room: %{id: "healthy-room", status: "active", description: "", tags: "auth,api",
                updated_at: ~N[2026-03-29 14:00:00]},
        active: false,
        count: 0
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      refute html =~ "needs synthesis"
      refute html =~ ~r/bg-red-500.*stale/
    end

    test "shows truncated latest_id as cursor" do
      assigns = %{
        room: %{id: "cursor-room", status: "active", description: "", tags: "",
                updated_at: ~N[2026-03-29 14:00:00]},
        active: false,
        count: 0,
        latest_id: "019d0000-0000-7000-8000-abcdef123456"
      }

      html = render_component(&CouncilComponents.room_card/1, assigns)
      assert html =~ "019d0000"
      assert html =~ "cursor:"
    end

    test "no cursor shown when latest_id is nil" do
      assigns = %{
        room: %{id: "no-cursor-room", status: "active", description: "", tags: "",
                updated_at: ~N[2026-03-29 14:00:00]},
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
      assert html =~ "amber"
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
      assert html =~ "my-proj"
      assert html =~ "Elixir, Go"
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

  describe "message_bubble" do
    test "renders author, type, and content" do
      assigns = %{
        msg: %{
          id: "uuid-0001",
          author: "Claude",
          content: "Hello **world**",
          message_type: "thought",
          reply_to: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "Claude"
      assert html =~ "thought"
      assert html =~ "world"
    end

    test "renders reply_to badge" do
      assigns = %{
        msg: %{
          id: "uuid-0002",
          author: "Gemini",
          content: "Reply here",
          message_type: "review",
          reply_to: "uuid-0001",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "re: #uuid-000"
    end

    test "no reply badge when reply_to is empty" do
      assigns = %{
        msg: %{
          id: "uuid-0003",
          author: "Claude",
          content: "No reply",
          message_type: "message",
          reply_to: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      refute html =~ "re: #"
    end

    test "renders critique type" do
      assigns = %{
        msg: %{
          id: "uuid-0004",
          author: "Gemini",
          content: "This is flawed",
          message_type: "critique",
          reply_to: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "critique"
    end

    test "renders synthesis type with purple and amber classes" do
      assigns = %{
        msg: %{
          id: "uuid-synth",
          author: "Claude",
          content: "Synthesized insight",
          message_type: "synthesis",
          reply_to: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "synthesis"
      assert html =~ "purple"
      assert html =~ "amber"
    end

    test "renders emoji reaction badges" do
      assigns = %{
        msg: %{
          id: "uuid-react",
          author: "Claude",
          content: "Nice work",
          message_type: "message",
          reply_to: "",
          reactions: ~s({"👍": ["gemini", "gpt"]}),
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "👍"
      assert html =~ "gemini, gpt"
    end

    test "renders emoji picker trigger button" do
      assigns = %{
        msg: %{
          id: "uuid-picker",
          author: "Claude",
          content: "React to this",
          message_type: "message",
          reply_to: "",
          reactions: "{}",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "EmojiPicker"
      assert html =~ "emoji-picker-uuid-picker"
    end

    test "reaction badge phx-click sends react event" do
      assigns = %{
        msg: %{
          id: "uuid-react2",
          author: "Gemini",
          content: "Click the reaction",
          message_type: "message",
          reply_to: "",
          reactions: ~s({"🎉": ["claude"]}),
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ ~s(phx-click="react")
      assert html =~ "uuid-react2"
      assert html =~ "🎉"
    end

    test "renders copy button with message data" do
      assigns = %{
        msg: %{
          id: "uuid-0007",
          author: "Claude",
          content: "Important finding",
          message_type: "decision",
          reply_to: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "copy-msg-uuid-0007"
      assert html =~ "CopyMessage"
      assert html =~ "Important finding"
      assert html =~ "Claude"
      assert html =~ "decision"
    end

    test "copy button data includes message id" do
      assigns = %{
        msg: %{
          id: "uuid-0099",
          author: "GPT",
          content: "Some content",
          message_type: "thought",
          reply_to: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "#uuid-0099"
    end
  end

  describe "summary_block" do
    test "renders summary content" do
      assigns = %{
        msg: %{
          id: 1,
          content: "Summary of discussion",
          timestamp: ~N[2026-03-29 14:00:00]
        },
        collapsed: false
      }

      html = render_component(&CouncilComponents.summary_block/1, assigns)
      assert html =~ "Summary"
      assert html =~ "Summary of discussion"
    end

    test "renders collapsed state" do
      assigns = %{
        msg: %{
          id: 1,
          content: "Summary of discussion",
          timestamp: ~N[2026-03-29 14:00:00]
        },
        collapsed: true
      }

      html = render_component(&CouncilComponents.summary_block/1, assigns)
      assert html =~ "expand"
    end
  end
end
