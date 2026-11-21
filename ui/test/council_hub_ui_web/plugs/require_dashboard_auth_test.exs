defmodule CouncilHubUiWeb.Plugs.RequireDashboardAuthTest do
  # Not async: toggles the global :auth_token application env.
  use CouncilHubUiWeb.ConnCase, async: false

  @token "test-token-0123456789abcdef"

  # A fake secret with colons in it; the scan flags a literal that sits next to the word "token".

  @colon_secret "pw:with:colons:0123456789abcdef"
  @outside {172, 17, 0, 1}

  defp with_token(token, fun) do
    previous = Application.get_env(:council_hub_ui, :auth_token)
    Application.put_env(:council_hub_ui, :auth_token, token)

    try do
      fun.()
    after
      case previous do
        nil -> Application.delete_env(:council_hub_ui, :auth_token)
        val -> Application.put_env(:council_hub_ui, :auth_token, val)
      end
    end
  end

  defp outside(conn), do: %{conn | remote_ip: @outside}

  defp basic(conn, user, password),
    do: put_req_header(conn, "authorization", "Basic " <> Base.encode64("#{user}:#{password}"))

  defp assert_denied(conn) do
    assert conn.status == 401
    assert get_resp_header(conn, "www-authenticate") == [~s(Basic realm="Council Hub")]
    assert conn.resp_body =~ "Authentication required"
    refute conn.resp_body =~ ~r/csrf|phx-session/i
    assert get_resp_header(conn, "set-cookie") == []
  end

  test "token off: an outside request is open", %{conn: conn} do
    with_token(nil, fn ->
      assert conn |> outside() |> get("/status") |> Map.fetch!(:status) == 200
    end)
  end

  test "token empty string counts as off", %{conn: conn} do
    with_token("", fn ->
      assert conn |> outside() |> get("/status") |> Map.fetch!(:status) == 200
    end)
  end

  test "no header is 401", %{conn: conn} do
    with_token(@token, fn -> conn |> outside() |> get("/status") |> assert_denied() end)
  end

  test "wrong scheme is 401", %{conn: conn} do
    with_token(@token, fn ->
      conn
      |> outside()
      |> put_req_header("authorization", "Bearer " <> @token)
      |> get("/status")
      |> assert_denied()
    end)
  end

  test "malformed basic payloads are 401", %{conn: conn} do
    with_token(@token, fn ->
      conn
      |> outside()
      |> put_req_header("authorization", "Basic !!!not-base64")
      |> get("/status")
      |> assert_denied()

      conn
      |> outside()
      |> put_req_header("authorization", "Basic " <> Base.encode64("no-colon-here"))
      |> get("/status")
      |> assert_denied()
    end)
  end

  test "wrong password is 401", %{conn: conn} do
    with_token(@token, fn ->
      conn
      |> outside()
      |> basic("anyone", "wrong-token-0123456789")
      |> get("/status")
      |> assert_denied()
    end)
  end

  test "the token as username with a wrong password is 401", %{conn: conn} do
    with_token(@token, fn ->
      conn |> outside() |> basic(@token, "x") |> get("/status") |> assert_denied()
    end)
  end

  test "right password with any username is 200", %{conn: conn} do
    with_token(@token, fn ->
      for user <- ["", "alice", "admin"] do
        resp = conn |> outside() |> basic(user, @token) |> get("/status")
        assert resp.status == 200
      end
    end)
  end

  test "a password containing a colon still works", %{conn: conn} do
    with_token(@colon_secret, fn ->
      resp = conn |> outside() |> basic("u", @colon_secret) |> get("/status")
      assert resp.status == 200
    end)
  end

  test "every browser route is protected, including export", %{conn: conn} do
    with_token(@token, fn ->
      for path <- [
            "/",
            "/status",
            "/notebook",
            "/skills",
            "/settings",
            "/rooms/x",
            "/rooms/x/export"
          ] do
        conn |> outside() |> get(path) |> assert_denied()
      end
    end)
  end

  test "loopback is exempt (IPv4, IPv6, IPv4-mapped)", %{conn: conn} do
    with_token(@token, fn ->
      for ip <- [{127, 0, 0, 1}, {0, 0, 0, 0, 0, 0, 0, 1}, {0, 0, 0, 0, 0, 65535, 32512, 1}] do
        resp = get(%{conn | remote_ip: ip}, "/status")
        assert resp.status == 200
      end
    end)
  end

  test "a non-loopback unauthenticated GET of / carries no session cookie and no token", %{
    conn: conn
  } do
    with_token(@token, fn ->
      resp = conn |> outside() |> get("/")
      assert_denied(resp)
      refute resp.resp_body =~ "csrf"
      refute resp.resp_body =~ "data-phx-session"
    end)
  end

  test "an authenticated GET of / embeds the LiveView session token", %{conn: conn} do
    with_token(@token, fn ->
      resp = conn |> outside() |> basic("u", @token) |> get("/")
      assert resp.status == 200
      assert resp.resp_body =~ "data-phx-session"
    end)
  end

  test "internal cluster API stays on its own localhost-only rule", %{conn: conn} do
    with_token(@token, fn ->
      # Loopback caller (the Go server): allowed without credentials.
      assert (conn |> get("/api/internal/cluster/nodes")).status == 200
      # Outside caller: refused by RestrictLocalhost, not by the Basic plug.
      resp = conn |> outside() |> basic("u", @token) |> get("/api/internal/cluster/nodes")
      assert resp.status == 403
      assert get_resp_header(resp, "www-authenticate") == []
    end)
  end

  test "cluster nodes reports auth_enabled as a boolean, never the token", %{conn: conn} do
    with_token(@token, fn ->
      body = conn |> get("/api/internal/cluster/nodes") |> json_response(200)
      assert body["auth_enabled"] == true
      refute Jason.encode!(body) =~ @token
    end)

    with_token(nil, fn ->
      body = conn |> get("/api/internal/cluster/nodes") |> json_response(200)
      assert body["auth_enabled"] == false
    end)
  end
end
