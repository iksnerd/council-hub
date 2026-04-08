defmodule CouncilHubUi.McpClient do
  @moduledoc """
  Thin HTTP client for calling tools on the Go MCP server.
  The Go server must be running in HTTP transport mode (COUNCIL_TRANSPORT=http).
  """

  require Logger

  @default_url "http://127.0.0.1:3001/mcp"

  defp mcp_url do
    Application.get_env(:council_hub_ui, :mcp_server_url, @default_url)
  end

  @doc "Add or remove an emoji reaction on a message."
  def react_to_message(message_id, emoji, author) do
    call_tool("react_to_message", %{message_id: message_id, emoji: emoji, author: author})
  end

  @doc "Set the status of a room (active | paused | resolved)."
  def signal_status(room_id, status) do
    call_tool("signal_status", %{room_id: room_id, status: status})
  end

  @doc "Archive a room — exports transcript to markdown file."
  def archive_room(room_id) do
    call_tool("archive_room", %{room_id: room_id})
  end

  @doc "Run the Knowledge Linter on a room and return its health report."
  def check_room_health(room_id) do
    call_tool("check_room_health", %{room_id: room_id})
  end

  @doc "Update tags on a room (CSV string, overwrites existing tags)."
  def update_room_tags(room_id, tags) do
    call_tool("update_room", %{room_id: room_id, tags: tags})
  end

  @doc "List all archived rooms. Returns {:ok, json_text} or {:error, reason}."
  def list_archives do
    call_tool_data("list_archives", %{})
  end

  @doc "Read the archived transcript for a room. Returns {:ok, markdown_text} or {:error, reason}."
  def read_archive(room_id) do
    call_tool_data("read_archive", %{room_id: room_id})
  end

  # --- Private ---

  defp call_tool(name, arguments) do
    body =
      Jason.encode!(%{
        jsonrpc: "2.0",
        id: System.unique_integer([:positive]),
        method: "tools/call",
        params: %{name: name, arguments: arguments}
      })

    url = String.to_charlist(mcp_url())

    headers = [
      {~c"Content-Type", ~c"application/json"},
      {~c"Accept", ~c"application/json, text/event-stream"}
    ]

    :httpc.request(:post, {url, headers, ~c"application/json", body}, [{:timeout, 8000}], [])
    |> handle_response()
  rescue
    e ->
      Logger.warning("McpClient error calling #{name}: #{inspect(e)}")
      {:error, :request_failed}
  end

  defp handle_response({:ok, {{_http, status, _reason}, _headers, _body}})
       when status in 200..299 do
    :ok
  end

  defp handle_response({:ok, {{_http, status, _reason}, _headers, body}}) do
    Logger.warning("McpClient unexpected status #{status}: #{inspect(body)}")
    {:error, {:http_error, status}}
  end

  defp handle_response({:error, reason}) do
    Logger.warning("McpClient request failed: #{inspect(reason)}")
    {:error, reason}
  end

  defp call_tool_data(name, arguments) do
    body =
      Jason.encode!(%{
        jsonrpc: "2.0",
        id: System.unique_integer([:positive]),
        method: "tools/call",
        params: %{name: name, arguments: arguments}
      })

    url = String.to_charlist(mcp_url())

    headers = [
      {~c"Content-Type", ~c"application/json"},
      {~c"Accept", ~c"application/json, text/event-stream"}
    ]

    :httpc.request(:post, {url, headers, ~c"application/json", body}, [{:timeout, 8000}], [])
    |> handle_data_response(name)
  rescue
    e ->
      Logger.warning("McpClient error calling #{name}: #{inspect(e)}")
      {:error, :request_failed}
  end

  defp handle_data_response({:ok, {{_http, status, _}, _headers, body}}, _name)
       when status in 200..299 do
    case Jason.decode(to_string(body)) do
      {:ok, %{"result" => %{"content" => [%{"text" => text} | _]}}} ->
        {:ok, text}

      {:ok, other} ->
        Logger.warning("McpClient unexpected response shape: #{inspect(other)}")
        {:error, :unexpected_response}

      {:error, _} ->
        {:error, :json_decode_failed}
    end
  end

  defp handle_data_response({:ok, {{_http, status, _}, _, body}}, name) do
    Logger.warning("McpClient #{name} unexpected status #{status}: #{inspect(body)}")
    {:error, {:http_error, status}}
  end

  defp handle_data_response({:error, reason}, _name) do
    Logger.warning("McpClient request failed: #{inspect(reason)}")
    {:error, reason}
  end
end
