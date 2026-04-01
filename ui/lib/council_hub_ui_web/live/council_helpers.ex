defmodule CouncilHubUiWeb.CouncilHelpers do
  @moduledoc false

  @known_authors %{
    "claude" => {"#C084FC", "bg-purple-900/30 border-purple-500/50 text-purple-300"},
    "gemini" => {"#60A5FA", "bg-blue-900/30 border-blue-500/50 text-blue-300"},
    "gpt" => {"#34D399", "bg-emerald-900/30 border-emerald-500/50 text-emerald-300"},
    "system" => {"#F59E0B", "bg-amber-900/30 border-amber-500/50 text-amber-300"},
    "admin" => {"#F472B6", "bg-pink-900/30 border-pink-500/50 text-pink-300"},
    "amp" => {"#22D3EE", "bg-cyan-900/30 border-cyan-500/50 text-cyan-300"}
  }

  @fallback_colors [
    {"#FB923C", "bg-orange-900/30 border-orange-500/50 text-orange-300"},
    {"#2DD4BF", "bg-teal-900/30 border-teal-500/50 text-teal-300"},
    {"#A78BFA", "bg-violet-900/30 border-violet-500/50 text-violet-300"},
    {"#FBBF24", "bg-yellow-900/30 border-yellow-500/50 text-yellow-300"},
    {"#F472B6", "bg-pink-900/30 border-pink-500/50 text-pink-300"},
    {"#818CF8", "bg-indigo-900/30 border-indigo-500/50 text-indigo-300"},
    {"#22D3EE", "bg-cyan-900/30 border-cyan-500/50 text-cyan-300"},
    {"#4ADE80", "bg-green-900/30 border-green-500/50 text-green-300"}
  ]

  @type_icons %{
    "message" => "hero-chat-bubble-left",
    "thought" => "hero-light-bulb",
    "decision" => "hero-check-badge",
    "code" => "hero-code-bracket",
    "review" => "hero-magnifying-glass",
    "action" => "hero-bolt",
    "critique" => "hero-exclamation-triangle",
    "error" => "hero-x-circle"
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
      "decision" -> "bg-green-500/15 text-green-400 border-green-500/30"
      "action"   -> "bg-blue-500/15 text-blue-400 border-blue-500/30"
      "critique" -> "bg-amber-500/15 text-amber-400 border-amber-500/30"
      "code"     -> "bg-purple-500/15 text-purple-400 border-purple-500/30"
      "review"   -> "bg-teal-500/15 text-teal-400 border-teal-500/30"
      "thought"  -> "bg-gray-700/50 text-gray-500 border-gray-600/30"
      _          -> "bg-gray-800/80 text-gray-400 border-gray-700/50"
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
      "active" -> "bg-green-500/20 text-green-400 border border-green-500/30"
      "paused" -> "bg-yellow-500/20 text-yellow-400 border border-yellow-500/30"
      "resolved" -> "bg-neutral-500/20 text-neutral-400 border border-neutral-500/30"
      _ -> "bg-neutral-500/20 text-neutral-400 border border-neutral-500/30"
    end
  end

  def status_dot_class(status) do
    case status do
      "active" -> "bg-green-400 animate-pulse"
      "paused" -> "bg-yellow-400"
      "resolved" -> "bg-neutral-400"
      _ -> "bg-neutral-400"
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

  def message_count_label(0), do: nil
  def message_count_label(count), do: "#{count}"

  def present?(nil), do: false
  def present?(""), do: false
  def present?(_), do: true
end
