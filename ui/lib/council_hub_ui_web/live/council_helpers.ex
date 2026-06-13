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

  # `code` is a retired message type (no longer offered for new posts), but its
  # icon/color/abbrev mappings are kept here so historical `code` messages still
  # render with their badge.
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
    "synthesis" => "hero-beaker",
    "note" => "hero-document-text",
    "plan" => "hero-clipboard-document-list"
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
      "action" -> "bg-[var(--ch-raised)] text-[var(--ch-text-lo)]"
      "critique" -> "bg-amber-500/10 text-amber-400/80"
      "code" -> "bg-violet-500/10 text-violet-400/80"
      "review" -> "bg-[var(--ch-raised)] text-[var(--ch-text-lo)]"
      "thought" -> "bg-[var(--ch-raised)] text-[var(--ch-text-xs)]"
      "draft" -> "bg-[var(--ch-raised)] text-[var(--ch-text-lo)]"
      "synthesis" -> "bg-purple-500/10 text-purple-300/80"
      "note" -> "bg-sky-500/10 text-sky-300/80"
      "plan" -> "bg-teal-500/10 text-teal-300/80"
      _ -> "bg-[var(--ch-raised)] text-[var(--ch-text-xs)]"
    end
  end

  # MIRROR: this resolver is duplicated in the Go server
  # (mcp-server/internal/council/commits.go — ResolveCommitRefs, commitBaseURL)
  # because the BEAM and Go render the same content independently and can't share
  # code. The two must stay byte-for-byte identical (regexes, short-SHA rule, URL
  # normalization). Their paired tests are load-bearing — if you change one side,
  # change the other and update both test files.
  @commit_ref_re ~r/\{sha:([0-9a-fA-F]{7,40})\}/
  @owner_repo_re ~r/^[\w.-]+\/[\w.-]+$/
  @scp_remote_re ~r/^[\w.-]+@([\w.-]+):(.+)$/

  @doc """
  Resolves `{sha:<hash>}` tokens in message content into short-SHA markdown
  commit links when `repo` maps to a known host, or a bare `` `short` `` code
  span otherwise. Mirrors the Go server's render-time resolver (`commits.go`).
  Render-time only, read-only — the link is built purely from the SHA and repo.
  """
  def resolve_commit_refs(content, repo) when is_binary(content) do
    if String.contains?(content, "{sha:") do
      base = repo_url(repo)

      Regex.replace(@commit_ref_re, content, fn _full, hash ->
        short = String.slice(hash, 0, 7)

        if base == "",
          do: "`#{short}`",
          else: "[`#{short}`](#{base}/commit/#{hash})"
      end)
    else
      content
    end
  end

  def resolve_commit_refs(content, _repo), do: content

  @doc """
  Converts a room's repo reference (owner/repo, clone URL, or scp remote) into
  its canonical https base URL — e.g. `https://github.com/owner/repo`. Its
  "/commit/<sha>" path points at a single commit. Returns "" for empty or
  unrecognised values. Used both for {sha:...} links and the room-header link.
  """
  def repo_url(repo) when is_binary(repo) do
    repo =
      repo
      |> String.trim()
      |> String.replace_suffix("/", "")
      |> String.replace_suffix(".git", "")

    cond do
      repo == "" ->
        ""

      String.starts_with?(repo, "http://") or String.starts_with?(repo, "https://") ->
        repo

      Regex.match?(@owner_repo_re, repo) ->
        "https://github.com/#{repo}"

      true ->
        case Regex.run(@scp_remote_re, repo) do
          [_, host, path] -> "https://#{host}/#{path}"
          _ -> ""
        end
    end
  end

  def repo_url(_), do: ""

  def render_markdown(nil), do: ""

  def render_markdown(content) do
    # Message/room content is authored by untrusted LLM/human agents and rendered
    # with raw/1 in the templates. Earmark passes raw HTML through unchanged, so the
    # output must be sanitized to prevent stored XSS (e.g. <img onerror=…>, <script>).
    html =
      case Earmark.as_html(content, %Earmark.Options{smartypants: false}) do
        {:ok, html, _warnings} -> html
        {:error, html, _errors} -> html
      end

    HtmlSanitizeEx.markdown_html(html)
  end

  def status_badge_class(status) do
    case status do
      "active" ->
        "bg-emerald-500/10 text-emerald-400/80 border border-emerald-500/15"

      "paused" ->
        "bg-amber-500/10 text-amber-400/80 border border-amber-500/15"

      "resolved" ->
        "bg-[var(--ch-raised)] text-[var(--ch-text-xs)] border border-[var(--ch-border)]"

      _ ->
        "bg-[var(--ch-raised)] text-[var(--ch-text-xs)] border border-[var(--ch-border)]"
    end
  end

  def status_dot_class(status) do
    case status do
      "active" -> "bg-emerald-400 animate-pulse"
      "paused" -> "bg-amber-400"
      "resolved" -> "bg-[var(--ch-text-xs)]"
      _ -> "bg-[var(--ch-text-xs)]"
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
    "error" => "E",
    "note" => "N",
    "plan" => "Pl"
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
      needs_synthesis: "needs-synthesis" in tags,
      stale_pin: "stale-pin" in tags,
      stale_plan: "stale-plan" in tags,
      incoherent: "incoherent" in tags
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
