defmodule CouncilHubUiWeb.UserSocketOriginTest do
  # The release config sets check_origin: false for the whole endpoint, and a
  # socket inherits it unless it overrides it. These requests go through the
  # endpoint, so they prove the per-socket override is wired in.
  use CouncilHubUiWeb.ConnCase, async: true

  @path "/socket/websocket?vsn=2.0.0"

  test "a request with an Origin header is refused", %{conn: conn} do
    conn =
      conn
      |> put_req_header("origin", "http://web-page.example")
      |> get(@path)

    assert conn.status == 403
  end

  test "a same-host Origin is refused too, a browser never opens this socket", %{conn: conn} do
    conn =
      conn
      |> put_req_header("origin", "http://localhost")
      |> get(@path)

    assert conn.status == 403
  end

  test "a request with no Origin header is not refused by the origin check", %{conn: conn} do
    status =
      try do
        conn |> get(@path) |> Map.fetch!(:status)
      rescue
        # The test adapter cannot complete a real upgrade; reaching that point
        # means the origin check let the request through.
        _ -> :passed_origin_check
      end

    refute status == 403
  end
end
