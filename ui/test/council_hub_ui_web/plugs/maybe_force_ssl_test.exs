defmodule CouncilHubUiWeb.Plugs.MaybeForceSslTest do
  # Not async: toggles the global :force_ssl_enabled application env.
  use ExUnit.Case, async: false
  import Plug.Test

  alias CouncilHubUiWeb.Plugs.MaybeForceSsl

  defp with_force_ssl(enabled, fun) do
    previous = Application.get_env(:council_hub_ui, :force_ssl_enabled)
    Application.put_env(:council_hub_ui, :force_ssl_enabled, enabled)

    try do
      fun.()
    after
      case previous do
        nil -> Application.delete_env(:council_hub_ui, :force_ssl_enabled)
        val -> Application.put_env(:council_hub_ui, :force_ssl_enabled, val)
      end
    end
  end

  defp call(host) do
    conn(:get, "http://#{host}/some/path")
    |> MaybeForceSsl.call(MaybeForceSsl.init([]))
  end

  test "disabled (default) — plain http passes through untouched" do
    with_force_ssl(false, fn ->
      conn = call("example.com")
      refute conn.halted
      assert conn.status == nil
    end)
  end

  test "unset config — passes through (off by default)" do
    previous = Application.get_env(:council_hub_ui, :force_ssl_enabled)
    Application.delete_env(:council_hub_ui, :force_ssl_enabled)

    try do
      conn = call("example.com")
      refute conn.halted
    after
      if previous != nil,
        do: Application.put_env(:council_hub_ui, :force_ssl_enabled, previous)
    end
  end

  test "enabled — redirects a non-localhost http request to https" do
    with_force_ssl(true, fn ->
      conn = call("example.com")
      assert conn.halted
      assert conn.status in [301, 302, 307, 308]
      assert [location] = Plug.Conn.get_resp_header(conn, "location")
      assert String.starts_with?(location, "https://example.com")
    end)
  end

  test "enabled — a request already https via X-Forwarded-Proto is not redirected" do
    with_force_ssl(true, fn ->
      conn =
        conn(:get, "http://example.com/some/path")
        |> Plug.Conn.put_req_header("x-forwarded-proto", "https")
        |> MaybeForceSsl.call(MaybeForceSsl.init([]))

      refute conn.halted
    end)
  end

  test "enabled — never redirects localhost (Go internal API and healthchecks)" do
    with_force_ssl(true, fn ->
      for host <- ["localhost", "127.0.0.1"] do
        conn = call(host)
        refute conn.halted, "#{host} must not be redirected"
        assert conn.status == nil
      end
    end)
  end
end
