defmodule CouncilHubUiWeb.CouncilComponentsMessageTest do
  use CouncilHubUiWeb.ConnCase

  import Phoenix.LiveViewTest

  alias CouncilHubUiWeb.CouncilComponents

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

    test "reply badge is a button with ScrollToMessage hook and full reply_to id" do
      assigns = %{
        msg: %{
          id: "uuid-0002",
          author: "Gemini",
          content: "Reply here",
          message_type: "message",
          reply_to: "uuid-0001-full-id",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ ~s(phx-hook="ScrollToMessage")
      assert html =~ ~s(data-reply-to="uuid-0001-full-id")
      assert html =~ ~s(id="reply-btn-uuid-0002")
    end

    test "renders supersedes badge linking the replaced message" do
      assigns = %{
        msg: %{
          id: "uuid-v2",
          author: "Claude",
          content: "v2 synthesis",
          message_type: "synthesis",
          reply_to: "",
          supersedes: "uuid-v1-full-id",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "supersedes #uuid-v1"
      assert html =~ ~s(id="supersedes-btn-uuid-v2")
      assert html =~ ~s(data-reply-to="uuid-v1-full-id")
    end

    test "no supersedes badge when supersedes is empty" do
      assigns = %{
        msg: %{
          id: "uuid-plain",
          author: "Claude",
          content: "no supersedes",
          message_type: "synthesis",
          reply_to: "",
          supersedes: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      refute html =~ "supersedes #"
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

    test "renders synthesis type with purple classes" do
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

    test "pinned message renders border and PIN badge" do
      assigns = %{
        msg: %{
          id: "uuid-pinned",
          author: "Claude",
          content: "Important pinned message",
          message_type: "synthesis",
          reply_to: "",
          pinned: true,
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "PIN"
      assert html =~ "ch-border"
    end

    test "renders @mention tags when mentions present" do
      assigns = %{
        msg: %{
          id: "uuid-mentions",
          author: "Gemini",
          content: "Hey team",
          message_type: "message",
          reply_to: "",
          mentions: "claude,gpt",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      assert html =~ "@claude"
      assert html =~ "@gpt"
    end

    test "no mention tags when mentions empty" do
      assigns = %{
        msg: %{
          id: "uuid-no-mentions",
          author: "Claude",
          content: "Just a message",
          message_type: "message",
          reply_to: "",
          mentions: "",
          timestamp: ~N[2026-03-29 14:00:00]
        }
      }

      html = render_component(&CouncilComponents.message_bubble/1, assigns)
      refute html =~ "@"
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
      assert html =~ ~s(aria-expanded="false")
    end

    test "renders reactions on summary block" do
      assigns = %{
        msg: %{
          id: "uuid-sum-react",
          content: "Great summary",
          reactions: ~s({"👍": ["claude", "gemini"]}),
          timestamp: ~N[2026-03-29 14:00:00]
        },
        collapsed: false
      }

      html = render_component(&CouncilComponents.summary_block/1, assigns)
      assert html =~ "👍"
      assert html =~ "claude, gemini"
    end

    test "no reactions section when summary has no reactions" do
      assigns = %{
        msg: %{
          id: "uuid-sum-noreact",
          content: "Summary no reactions",
          timestamp: ~N[2026-03-29 14:00:00]
        },
        collapsed: false
      }

      html = render_component(&CouncilComponents.summary_block/1, assigns)
      refute html =~ "👍"
    end
  end
end
