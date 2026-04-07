defmodule CouncilHubUi.McpClientTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.McpClient

  describe "react_to_message/3" do
    test "returns error tuple when MCP server is unreachable" do
      # Point at a port nothing is listening on
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:19999/mcp")

      result = McpClient.react_to_message("some-id", "👍", "test-user")

      assert {:error, _reason} = result
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end

    test "returns error tuple on invalid URL scheme" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://[invalid-host]/mcp")

      result = McpClient.react_to_message("some-id", "🎉", "test-user")

      assert {:error, _reason} = result
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end
  end
end
