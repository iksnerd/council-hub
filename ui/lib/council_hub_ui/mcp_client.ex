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

  @doc """
  Sends a react_to_message tool call to the MCP server.
  Returns :ok on success or {:error, reason} on failure.
  """
  def react_to_message(message_id, emoji, author) do
    body =
      Jason.encode!(%{
        jsonrpc: "2.0",
        id: System.unique_integer([:positive]),
        method: "tools/call",
        params: %{
          name: "react_to_message",
          arguments: %{message_id: message_id, emoji: emoji, author: author}
        }
      })

    url = String.to_charlist(mcp_url())
    headers = [
      {~c"Content-Type", ~c"application/json"},
      {~c"Accept", ~c"application/json, text/event-stream"}
    ]

    :httpc.request(:post, {url, headers, ~c"application/json", body}, [{:timeout, 5000}], [])
    |> handle_response()
  rescue
    e ->
      Logger.warning("McpClient error: #{inspect(e)}")
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
end
