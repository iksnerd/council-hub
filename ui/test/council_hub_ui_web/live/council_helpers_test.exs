defmodule CouncilHubUiWeb.CouncilHelpersTest do
  use ExUnit.Case, async: true

  alias CouncilHubUiWeb.CouncilHelpers

  # -- author_color / author_hex / author_classes --

  test "known author gets assigned color" do
    {hex, classes} = CouncilHelpers.author_color("Claude")
    assert hex == "#a78bfa"
    assert String.contains?(classes, "violet")
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
    assert CouncilHelpers.author_hex("Claude") == "#a78bfa"
  end

  test "author_classes returns CSS classes" do
    classes = CouncilHelpers.author_classes("Claude")
    assert String.contains?(classes, "border-")
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

  test "render_markdown strips XSS from untrusted content" do
    malicious = "<img src=x onerror=\"steal()\"> <script>danger()</script>"
    html = CouncilHelpers.render_markdown(malicious)
    # The executable surfaces — the event-handler attribute and the script tag —
    # must be stripped. Inert leftover text is harmless.
    refute String.contains?(html, "onerror")
    refute String.contains?(html, "<script")
  end

  test "render_markdown keeps safe markdown after sanitizing" do
    html = CouncilHelpers.render_markdown("**bold** and [link](/local-path)")
    assert String.contains?(html, "<strong>bold</strong>")
    assert String.contains?(html, "<a")
    assert String.contains?(html, "/local-path")
  end

  # -- status_badge_class --

  test "status badge classes" do
    assert String.contains?(CouncilHelpers.status_badge_class("active"), "emerald")
    assert String.contains?(CouncilHelpers.status_badge_class("paused"), "amber")
    assert String.contains?(CouncilHelpers.status_badge_class("resolved"), "ch-raised")
    assert String.contains?(CouncilHelpers.status_badge_class("unknown"), "ch-raised")
  end

  # -- status_dot_class --

  test "status dot classes" do
    assert String.contains?(CouncilHelpers.status_dot_class("active"), "emerald")
    assert String.contains?(CouncilHelpers.status_dot_class("active"), "pulse")
    assert String.contains?(CouncilHelpers.status_dot_class("paused"), "amber")
    assert String.contains?(CouncilHelpers.status_dot_class("resolved"), "ch-text-xs")
    assert String.contains?(CouncilHelpers.status_dot_class("other"), "ch-text-xs")
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

  # -- type_color --

  test "type_color decision is emerald" do
    assert String.contains?(CouncilHelpers.type_color("decision"), "emerald")
  end

  test "type_color action uses CSS variable" do
    assert String.contains?(CouncilHelpers.type_color("action"), "ch-raised")
  end

  test "type_color critique is amber" do
    assert String.contains?(CouncilHelpers.type_color("critique"), "amber")
  end

  test "type_color code is violet" do
    assert String.contains?(CouncilHelpers.type_color("code"), "violet")
  end

  test "type_color review uses CSS variable" do
    assert String.contains?(CouncilHelpers.type_color("review"), "ch-raised")
  end

  test "type_color thought uses CSS variable" do
    assert String.contains?(CouncilHelpers.type_color("thought"), "ch-raised")
  end

  test "type_color synthesis uses purple" do
    result = CouncilHelpers.type_color("synthesis")
    assert String.contains?(result, "purple")
  end

  test "type_color draft uses CSS variable" do
    assert String.contains?(CouncilHelpers.type_color("draft"), "ch-raised")
  end

  test "type_color unknown falls back to CSS variable" do
    result = CouncilHelpers.type_color("unknown")
    assert String.contains?(result, "ch-raised")
  end

  test "type_color nil falls back to CSS variable" do
    result = CouncilHelpers.type_color(nil)
    assert String.contains?(result, "ch-raised")
  end

  # -- type_icon (synthesis) --

  test "type_icon synthesis is beaker" do
    assert CouncilHelpers.type_icon("synthesis") == "hero-beaker"
  end

  # -- parse_reactions --

  test "parse_reactions nil returns empty map" do
    assert CouncilHelpers.parse_reactions(nil) == %{}
  end

  test "parse_reactions empty string returns empty map" do
    assert CouncilHelpers.parse_reactions("") == %{}
  end

  test "parse_reactions empty JSON object returns empty map" do
    assert CouncilHelpers.parse_reactions("{}") == %{}
  end

  test "parse_reactions decodes emoji map" do
    json = ~s({"👍": ["claude", "gemini"], "🎉": ["admin"]})
    result = CouncilHelpers.parse_reactions(json)
    assert Map.get(result, "👍") == ["claude", "gemini"]
    assert Map.get(result, "🎉") == ["admin"]
  end

  test "parse_reactions invalid JSON returns empty map" do
    assert CouncilHelpers.parse_reactions("not json") == %{}
  end

  # -- room_health_flags --

  test "room_health_flags no health tags" do
    flags = CouncilHelpers.room_health_flags(%{tags: "feature,auth"})
    refute flags.stale
    refute flags.needs_synthesis
  end

  test "room_health_flags detects stale" do
    flags = CouncilHelpers.room_health_flags(%{tags: "stale,auth"})
    assert flags.stale
    refute flags.needs_synthesis
  end

  test "room_health_flags detects needs-synthesis" do
    flags = CouncilHelpers.room_health_flags(%{tags: "needs-synthesis"})
    refute flags.stale
    assert flags.needs_synthesis
  end

  test "room_health_flags detects both flags" do
    flags = CouncilHelpers.room_health_flags(%{tags: "stale,needs-synthesis"})
    assert flags.stale
    assert flags.needs_synthesis
  end

  test "room_health_flags nil tags" do
    flags = CouncilHelpers.room_health_flags(%{tags: nil})
    refute flags.stale
    refute flags.needs_synthesis
  end

  # -- present? --

  test "present? nil is false" do
    refute CouncilHelpers.present?(nil)
  end

  test "present? empty string is false" do
    refute CouncilHelpers.present?("")
  end

  test "present? non-empty string is true" do
    assert CouncilHelpers.present?("hello")
  end

  # -- short_node --

  test "short_node extracts name from node string" do
    assert CouncilHelpers.short_node("council_hub@my-node") == "council_hub"
  end

  test "short_node extracts name from node string with IP" do
    assert CouncilHelpers.short_node("council_hub@10.0.0.5") == "council_hub"
  end

  test "short_node returns string unchanged when no @ present" do
    assert CouncilHelpers.short_node("noatsign") == "noatsign"
  end

  test "short_node returns nil for nil" do
    assert CouncilHelpers.short_node(nil) == nil
  end

  # -- author_prose_class --

  test "author_prose_class returns border class" do
    classes = CouncilHelpers.author_prose_class("Claude")
    assert String.contains?(classes, "border-")
    assert String.contains?(classes, "violet")
  end

  # -- parse_mentions --

  test "parse_mentions nil returns empty list" do
    assert CouncilHelpers.parse_mentions(nil) == []
  end

  test "parse_mentions empty string returns empty list" do
    assert CouncilHelpers.parse_mentions("") == []
  end

  test "parse_mentions splits comma-separated mentions" do
    assert CouncilHelpers.parse_mentions("claude,gemini") == ["claude", "gemini"]
  end

  test "parse_mentions trims whitespace" do
    assert CouncilHelpers.parse_mentions(" claude , gemini ") == ["claude", "gemini"]
  end

  test "parse_mentions filters empty segments" do
    assert CouncilHelpers.parse_mentions("claude,,gemini") == ["claude", "gemini"]
  end

  # -- format_type_counts --

  test "format_type_counts empty map returns nil" do
    assert CouncilHelpers.format_type_counts(%{}) == nil
  end

  test "format_type_counts formats counts in descending order" do
    result = CouncilHelpers.format_type_counts(%{"decision" => 3, "thought" => 1})
    assert String.contains?(result, "D:3")
    assert String.contains?(result, "T:1")
    # decision has more count, should come first
    assert String.starts_with?(result, "D:")
  end

  test "format_type_counts unknown type uses first letter" do
    result = CouncilHelpers.format_type_counts(%{"custom_type" => 2})
    assert String.contains?(result, ":2")
  end

  # -- format_time_range --

  test "format_time_range nil first returns nil" do
    assert CouncilHelpers.format_time_range(nil, ~N[2026-03-29 14:00:00]) == nil
  end

  test "format_time_range nil last returns nil" do
    assert CouncilHelpers.format_time_range(~N[2026-03-29 12:00:00], nil) == nil
  end

  test "format_time_range both nil returns nil" do
    assert CouncilHelpers.format_time_range(nil, nil) == nil
  end

  test "format_time_range formats range" do
    first = ~N[2026-03-29 12:00:00]
    last = ~N[2026-03-29 14:30:00]
    result = CouncilHelpers.format_time_range(first, last)
    assert String.contains?(result, "→")
    assert String.contains?(result, "12:00")
    assert String.contains?(result, "14:30")
  end

  # -- node_host --

  test "node_host extracts host from node string" do
    assert CouncilHelpers.node_host("council_hub@10.0.0.5") == "10.0.0.5"
  end

  test "node_host extracts named host" do
    assert CouncilHelpers.node_host("council_hub@my-node") == "my-node"
  end

  test "node_host returns string unchanged when no @ present" do
    assert CouncilHelpers.node_host("noatsign") == "noatsign"
  end

  test "node_host returns nil for nil" do
    assert CouncilHelpers.node_host(nil) == nil
  end
end
