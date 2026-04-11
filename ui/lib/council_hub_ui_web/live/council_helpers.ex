defmodule CouncilHubUiWeb.CouncilHelpers do
  @moduledoc false

  @known_authors %{
    "claude" => {"#a78bfa", "border-violet-500/20"},
    "gemini" => {"#60a5fa", "border-blue-500/20"},
    "gpt" => {"#34d399", "border-emerald-500/20"},
    "system" => {"#fbbf24", "border-amber-500/20"},
    "admin" => {"#f472b6", "border-pink-500/20"},
    "amp" => {"#22d3ee", "border-cyan-500/20"}
  }

  @fallback_colors [
    {"#fb923c", "border-orange-500/20"},
    {"#2dd4bf", "border-teal-500/20"},
    {"#a78bfa", "border-violet-500/20"},
    {"#fbbf24", "border-amber-500/20"},
    {"#f472b6", "border-pink-500/20"},
    {"#818cf8", "border-indigo-500/20"},
    {"#22d3ee", "border-cyan-500/20"},
    {"#4ade80", "border-green-500/20"}
  ]

  @type_icons %{
    "message" => "hero-chat-bubble-left",
    "thought" => "hero-light-bulb",
    "draft" => "hero-pencil-square",
    "decision" => "hero-check-badge",
    "code" => "hero-code-bracket",
    "review" => "hero-magnifying-glass",
    "action" => "hero-bolt",
    "critique" => "hero-exclamation-triangle",
    "error" => "hero-x-circle",
    "synthesis" => "hero-beaker"
  }

  def author_color(author) do
    normalized = String.downcase(author || "unknown")

    case Map.get(@known_authors, normalized) do
      nil ->
        index = :erlang.phash2(normalized, length(@fallback_colors))
        Enum.at(@fallback_colors, index)

      color ->
        color
    end
  end

  def author_hex(author) do
    {hex, _classes} = author_color(author)
    hex
  end

  def author_classes(author) do
    {_hex, classes} = author_color(author)
    classes
  end

  def author_prose_class(author) do
    {_hex, border_class} = author_color(author)
    border_class
  end

  def author_initials(nil), do: "?"

  def author_initials(author) do
    author
    |> String.split(~r/[\s_-]+/)
    |> Enum.take(2)
    |> Enum.map(&String.first/1)
    |> Enum.join()
    |> String.upcase()
  end

  def type_icon(type) do
    Map.get(@type_icons, type || "message", "hero-chat-bubble-left")
  end

  def type_label(nil), do: "message"
  def type_label(""), do: "message"
  def type_label(type), do: type

  def type_color(type) do
    case type do
      "decision" -> "bg-emerald-500/10 text-emerald-400/80"
      "action" -> "bg-sky-500/10 text-sky-400/80"
      "critique" -> "bg-amber-500/10 text-amber-400/80"
      "code" -> "bg-violet-500/10 text-violet-400/80"
      "review" -> "bg-teal-500/10 text-teal-400/80"
      "thought" -> "bg-slate-700/30 text-slate-500"
      "draft" -> "bg-blue-500/10 text-blue-300/80"
      "synthesis" -> "bg-purple-500/10 text-purple-300/80"
      _ -> "bg-slate-800/40 text-slate-500"
    end
  end

  def render_markdown(nil), do: ""

  def render_markdown(content) do
    case Earmark.as_html(content, %Earmark.Options{smartypants: false}) do
      {:ok, html, _warnings} -> html
      {:error, html, _errors} -> html
    end
  end

  def status_badge_class(status) do
    case status do
      "active" -> "bg-emerald-500/10 text-emerald-400/80 border border-emerald-500/15"
      "paused" -> "bg-amber-500/10 text-amber-400/80 border border-amber-500/15"
      "resolved" -> "bg-slate-500/10 text-slate-400/80 border border-slate-500/15"
      _ -> "bg-slate-500/10 text-slate-400/80 border border-slate-500/15"
    end
  end

  def status_dot_class(status) do
    case status do
      "active" -> "bg-emerald-400 animate-pulse"
      "paused" -> "bg-amber-400"
      "resolved" -> "bg-slate-500"
      _ -> "bg-slate-500"
    end
  end

  def format_timestamp(nil), do: ""

  def format_timestamp(dt) do
    Calendar.strftime(dt, "%H:%M:%S")
  end

  def relative_time(nil), do: ""

  def relative_time(dt) do
    now = NaiveDateTime.utc_now()
    diff = NaiveDateTime.diff(now, dt, :second)

    cond do
      diff < 5 -> "just now"
      diff < 60 -> "#{diff}s ago"
      diff < 3600 -> "#{div(diff, 60)}m ago"
      diff < 86400 -> "#{div(diff, 3600)}h ago"
      true -> Calendar.strftime(dt, "%b %d, %H:%M")
    end
  end

  def format_date(nil), do: ""

  def format_date(dt) do
    Calendar.strftime(dt, "%Y-%m-%d %H:%M")
  end

  def truncate(nil, _), do: ""
  def truncate(str, max_length) when byte_size(str) <= max_length, do: str
  def truncate(str, max_length), do: String.slice(str, 0, max_length) <> "..."

  def parse_tags(nil), do: []
  def parse_tags(""), do: []

  def parse_tags(tags) do
    tags
    |> String.split(",")
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == ""))
  end

  def parse_mentions(nil), do: []
  def parse_mentions(""), do: []

  def parse_mentions(mentions) do
    mentions
    |> String.split(",")
    |> Enum.map(&String.trim/1)
    |> Enum.reject(&(&1 == ""))
  end

  @type_abbrevs %{
    "decision" => "D",
    "action" => "A",
    "code" => "C",
    "review" => "R",
    "thought" => "T",
    "draft" => "Dr",
    "synthesis" => "S",
    "critique" => "Cr",
    "message" => "M",
    "error" => "E"
  }

  def format_type_counts(type_counts) when map_size(type_counts) == 0, do: nil

  def format_type_counts(type_counts) do
    type_counts
    |> Enum.sort_by(fn {_k, v} -> -v end)
    |> Enum.map(fn {type, count} ->
      abbrev = Map.get(@type_abbrevs, type, type |> String.first() |> String.upcase())
      "#{abbrev}:#{count}"
    end)
    |> Enum.join(" ")
  end

  def format_time_range(nil, _), do: nil
  def format_time_range(_, nil), do: nil

  def format_time_range(first, last) do
    "#{Calendar.strftime(first, "%m/%d %H:%M")} → #{Calendar.strftime(last, "%m/%d %H:%M")}"
  end

  def parse_reactions(nil), do: %{}
  def parse_reactions(""), do: %{}
  def parse_reactions("{}"), do: %{}

  def parse_reactions(json) do
    case Jason.decode(json) do
      {:ok, map} when is_map(map) -> map
      _ -> %{}
    end
  end

  def room_health_flags(room) do
    tags = parse_tags(Map.get(room, :tags) || Map.get(room, "tags"))

    %{
      stale: "stale" in tags,
      needs_synthesis: "needs-synthesis" in tags
    }
  end

  def message_count_label(0), do: nil
  def message_count_label(count), do: "#{count}"

  def present?(nil), do: false
  def present?(""), do: false
  def present?(_), do: true

  def short_node(nil), do: nil

  def short_node(node_str) when is_binary(node_str) do
    case String.split(node_str, "@", parts: 2) do
      [name, _host] -> name
      _ -> node_str
    end
  end

  def node_host(nil), do: nil

  def node_host(node_str) when is_binary(node_str) do
    case String.split(node_str, "@", parts: 2) do
      [_name, host] -> host
      _ -> node_str
    end
  end
end
