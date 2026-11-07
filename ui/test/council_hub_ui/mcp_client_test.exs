defmodule CouncilHubUi.McpClientTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.McpClient

  describe "list_archives/0" do
    test "returns error tuple when MCP server is unreachable" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:19999/mcp")
      assert {:error, _} = McpClient.list_archives()
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end

    test "returns error tuple on invalid URL" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://[invalid-host]/mcp")
      assert {:error, _} = McpClient.list_archives()
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end
  end

  describe "read_archive/1" do
    test "returns error tuple when MCP server is unreachable" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:19999/mcp")
      assert {:error, _} = McpClient.read_archive("some-room")
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end
  end

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

  describe "backfill_embeddings/1" do
    test "returns error tuple when MCP server is unreachable" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:19999/mcp")
      assert {:error, _} = McpClient.backfill_embeddings()
      assert {:error, _} = McpClient.backfill_embeddings(true)
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end

    test "returns error tuple on invalid URL" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://[invalid-host]/mcp")
      assert {:error, _} = McpClient.backfill_embeddings()
    after
      Application.delete_env(:council_hub_ui, :mcp_server_url)
    end
  end

  describe "tailscale_status/0 and tailscale_apply/1" do
    setup do
      {:ok, agent} = Agent.start_link(fn -> %{reply: {200, ~s({"state":"off","config":{}})}} end)

      {:ok, server} =
        Bandit.start_link(
          plug: {CouncilHubUi.FakeTailscalePlug, agent: agent},
          port: 0,
          ip: :loopback
        )

      {:ok, {_ip, port}} = ThousandIsland.listener_info(server)
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:#{port}/mcp")
      on_exit(fn -> Application.delete_env(:council_hub_ui, :mcp_server_url) end)
      {:ok, agent: agent}
    end

    test "status returns the decoded view", %{agent: agent} do
      Agent.update(
        agent,
        &Map.put(&1, :reply, {200, ~s({"state":"running","url":"https://h.ts.net"})})
      )

      assert {:ok, %{"state" => "running", "url" => "https://h.ts.net"}} =
               McpClient.tailscale_status()

      assert {"GET", ""} = Agent.get(agent, & &1.last)
    end

    test "apply posts the config as JSON", %{agent: agent} do
      attrs = %{"enabled" => true, "hostname" => "hub", "access" => "owner", "tags" => []}
      assert {:ok, %{"state" => "off"}} = McpClient.tailscale_apply(attrs)
      assert {"POST", body} = Agent.get(agent, & &1.last)
      assert %{"enabled" => true, "hostname" => "hub", "access" => "owner"} = Jason.decode!(body)
    end

    test "a rejected config comes back as the Go server's message", %{agent: agent} do
      Agent.update(
        agent,
        &Map.put(&1, :reply, {400, "hostname must be 1-63 lowercase letters\n"})
      )

      assert {:error, "hostname must be 1-63 lowercase letters"} = McpClient.tailscale_apply(%{})
    end

    test "a server error is an error tuple, not a crash", %{agent: agent} do
      Agent.update(agent, &Map.put(&1, :reply, {500, "boom"}))
      assert {:error, {:http_error, 500}} = McpClient.tailscale_status()
    end

    test "unreachable server is an error tuple" do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:19999/mcp")
      assert {:error, _} = McpClient.tailscale_status()
    end
  end
end
