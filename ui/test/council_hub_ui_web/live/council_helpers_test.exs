defmodule CouncilHubUiWeb.CouncilHelpersTest do
  use ExUnit.Case, async: true

  alias CouncilHubUiWeb.CouncilHelpers

  # -- author_color / author_hex / author_classes --

  test "known author gets assigned color" do
    {hex, classes} = CouncilHelpers.author_color("Claude")
    assert hex == "#C084FC"
    assert String.contains?(classes, "purple")
  end

  test "known authors are case-insensitive" do
    assert CouncilHelpers.author_color("GEMINI") == CouncilHelpers.author_color("gemini")
  end

  test "unknown author gets deterministic fallback color" do
    {hex1, _} = CouncilHelpers.author_color("RandomAgent")
    {hex2, _} = CouncilHelpers.author_color("RandomAgent")
    assert hex1 == hex2
    assert hex1 != nil
  end

  test "author_hex returns hex string" do
    assert CouncilHelpers.author_hex("Claude") == "#C084FC"
  end

  test "author_classes returns CSS classes" do
    classes = CouncilHelpers.author_classes("Claude")
    assert String.contains?(classes, "bg-")
  end

  test "nil author gets fallback" do
    {hex, _} = CouncilHelpers.author_color(nil)
    assert hex != nil
  end

  # -- author_initials --

  test "single word initials" do
    assert CouncilHelpers.author_initials("Claude") == "C"
  end

  test "two word initials" do
    assert CouncilHelpers.author_initials("Claude Code") == "CC"
  end

  test "hyphenated initials" do
    assert CouncilHelpers.author_initials("Claude-Code") == "CC"
  end

  test "nil initials" do
    assert CouncilHelpers.author_initials(nil) == "?"
  end

  test "three word takes first two" do
    assert CouncilHelpers.author_initials("Claude Code Opus") == "CC"
  end

  # -- type_icon --

  test "known type icons" do
    assert CouncilHelpers.type_icon("message") == "hero-chat-bubble-left"
    assert CouncilHelpers.type_icon("thought") == "hero-light-bulb"
    assert CouncilHelpers.type_icon("decision") == "hero-check-badge"
    assert CouncilHelpers.type_icon("code") == "hero-code-bracket"
    assert CouncilHelpers.type_icon("review") == "hero-magnifying-glass"
    assert CouncilHelpers.type_icon("action") == "hero-bolt"
    assert CouncilHelpers.type_icon("critique") == "hero-exclamation-triangle"
    assert CouncilHelpers.type_icon("error") == "hero-x-circle"
  end

  test "unknown type falls back to message icon" do
    assert CouncilHelpers.type_icon("unknown") == "hero-chat-bubble-left"
  end

  test "nil type falls back to message icon" do
    assert CouncilHelpers.type_icon(nil) == "hero-chat-bubble-left"
  end

  # -- type_label --

  test "type_label returns type" do
    assert CouncilHelpers.type_label("thought") == "thought"
  end

  test "type_label nil defaults to message" do
    assert CouncilHelpers.type_label(nil) == "message"
  end

  test "type_label empty defaults to message" do
    assert CouncilHelpers.type_label("") == "message"
  end

  # -- render_markdown --

  test "render_markdown converts to HTML" do
    html = CouncilHelpers.render_markdown("**bold**")
    assert String.contains?(html, "<strong>bold</strong>")
  end

  test "render_markdown handles nil" do
    assert CouncilHelpers.render_markdown(nil) == ""
  end

  test "render_markdown handles code blocks" do
    html = CouncilHelpers.render_markdown("`inline code`")
    assert String.contains?(html, "<code")
  end

  # -- status_badge_class --

  test "status badge classes" do
    assert String.contains?(CouncilHelpers.status_badge_class("active"), "green")
    assert String.contains?(CouncilHelpers.status_badge_class("paused"), "yellow")
    assert String.contains?(CouncilHelpers.status_badge_class("resolved"), "neutral")
    assert String.contains?(CouncilHelpers.status_badge_class("unknown"), "neutral")
  end

  # -- status_dot_class --

  test "status dot classes" do
    assert String.contains?(CouncilHelpers.status_dot_class("active"), "green")
    assert String.contains?(CouncilHelpers.status_dot_class("active"), "pulse")
    assert String.contains?(CouncilHelpers.status_dot_class("paused"), "yellow")
    assert String.contains?(CouncilHelpers.status_dot_class("resolved"), "neutral")
    assert String.contains?(CouncilHelpers.status_dot_class("other"), "neutral")
  end

  # -- format_timestamp --

  test "format_timestamp formats time" do
    dt = ~N[2026-03-29 14:30:45]
    assert CouncilHelpers.format_timestamp(dt) == "14:30:45"
  end

  test "format_timestamp nil" do
    assert CouncilHelpers.format_timestamp(nil) == ""
  end

  # -- relative_time --

  test "relative_time just now" do
    assert CouncilHelpers.relative_time(NaiveDateTime.utc_now()) == "just now"
  end

  test "relative_time seconds ago" do
    dt = NaiveDateTime.add(NaiveDateTime.utc_now(), -30, :second)
    assert String.contains?(CouncilHelpers.relative_time(dt), "s ago")
  end

  test "relative_time minutes ago" do
    dt = NaiveDateTime.add(NaiveDateTime.utc_now(), -300, :second)
    assert String.contains?(CouncilHelpers.relative_time(dt), "m ago")
  end

  test "relative_time hours ago" do
    dt = NaiveDateTime.add(NaiveDateTime.utc_now(), -7200, :second)
    assert String.contains?(CouncilHelpers.relative_time(dt), "h ago")
  end

  test "relative_time days ago shows date" do
    dt = NaiveDateTime.add(NaiveDateTime.utc_now(), -172_800, :second)
    result = CouncilHelpers.relative_time(dt)
    # Should show month/day format, not relative
    refute String.contains?(result, "ago")
  end

  test "relative_time nil" do
    assert CouncilHelpers.relative_time(nil) == ""
  end

  # -- format_date --

  test "format_date" do
    assert CouncilHelpers.format_date(~N[2026-03-29 14:30:00]) == "2026-03-29 14:30"
  end

  test "format_date nil" do
    assert CouncilHelpers.format_date(nil) == ""
  end

  # -- truncate --

  test "truncate short string unchanged" do
    assert CouncilHelpers.truncate("hello", 10) == "hello"
  end

  test "truncate long string" do
    assert CouncilHelpers.truncate("hello world", 5) == "hello..."
  end

  test "truncate nil" do
    assert CouncilHelpers.truncate(nil, 10) == ""
  end

  # -- parse_tags --

  test "parse_tags splits comma-separated" do
    assert CouncilHelpers.parse_tags("auth,security,api") == ["auth", "security", "api"]
  end

  test "parse_tags trims whitespace" do
    assert CouncilHelpers.parse_tags(" auth , security ") == ["auth", "security"]
  end

  test "parse_tags nil" do
    assert CouncilHelpers.parse_tags(nil) == []
  end

  test "parse_tags empty string" do
    assert CouncilHelpers.parse_tags("") == []
  end

  test "parse_tags filters empty segments" do
    assert CouncilHelpers.parse_tags("auth,,api") == ["auth", "api"]
  end

  # -- message_count_label --

  test "message_count_label zero returns nil" do
    assert CouncilHelpers.message_count_label(0) == nil
  end

  test "message_count_label positive returns string" do
    assert CouncilHelpers.message_count_label(5) == "5"
  end
end
