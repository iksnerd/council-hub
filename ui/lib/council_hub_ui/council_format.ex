defmodule CouncilHubUi.CouncilFormat do
  @moduledoc "Transcript formatting helpers. Called via CouncilHubUi.Council facade."

  def format_transcript(room, messages) do
    header =
      ["# COUNCIL ROOM: #{room.id}"]
      |> maybe_append(room.project, &"**Project:** #{&1}")
      |> maybe_append(room.tech_stack, &"**Tech Stack:** #{&1}")
      |> then(&(&1 ++ ["**Topic:** #{room.description}", "**Status:** #{room.status}"]))
      |> maybe_append(room.tags, &"**Tags:** #{&1}")
      |> maybe_append(Map.get(room, :related_rooms, ""), &"**Related Rooms:** #{&1}")
      |> Enum.join("\n")

    system =
      if present?(room.system_prompt),
        do: "\n*Instructions: #{room.system_prompt}*\n---",
        else: ""

    body =
      messages
      |> Enum.map(&format_message/1)
      |> Enum.join("\n")

    footer =
      "\n---\n*SYSTEM: You are reading the Council log for \"#{room.id}\". Do not repeat previous points. Use `post_to_room` to contribute your next action.*"

    "#{header}\n---#{system}\n#{body}\n#{footer}\n"
  end

  defp format_message(%{is_summary: true} = msg) do
    "\n**[#{format_ts(msg.timestamp)}] SUMMARY:**\n#{msg.content}"
  end

  defp format_message(msg) do
    reply_to = Map.get(msg, :reply_to, "") || ""
    reply_tag = if reply_to != "", do: ", re: ##{String.slice(reply_to, 0, 8)}", else: ""
    ts = format_ts(msg.timestamp)

    cond do
      msg.message_type not in [nil, "", "message"] ->
        "\n**[#{ts}] #{msg.author} (#{msg.message_type}#{reply_tag}):**\n#{msg.content}"

      reply_to != "" ->
        "\n**[#{ts}] #{msg.author} (re: ##{String.slice(reply_to, 0, 8)}):**\n#{msg.content}"

      true ->
        "\n**[#{ts}] #{msg.author}:**\n#{msg.content}"
    end
  end

  defp format_ts(nil), do: ""
  defp format_ts(dt), do: Calendar.strftime(dt, "%Y-%m-%d %H:%M:%S")

  defp present?(nil), do: false
  defp present?(""), do: false
  defp present?(_), do: true

  defp maybe_append(lines, value, fmt) do
    if present?(value), do: lines ++ [fmt.(value)], else: lines
  end
end
