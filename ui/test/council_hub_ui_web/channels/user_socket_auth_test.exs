defmodule CouncilHubUiWeb.UserSocketAuthTest do
  # Not async: toggles the global :auth_token application env.
  use ExUnit.Case, async: false
  import Phoenix.ChannelTest
  import Plug.Conn
  import Phoenix.ConnTest, only: [build_conn: 0, get: 2]

  @endpoint CouncilHubUiWeb.Endpoint
  @token "test-token-0123456789abcdef"

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

  defp connect_with(headers),
    do: connect(CouncilHubUiWeb.UserSocket, %{}, connect_info: %{x_headers: headers})

  test "token off: connects with no header" do
    with_token(nil, fn -> assert {:ok, _} = connect_with([]) end)
  end

  test "token on: valid header connects" do
    with_token(@token, fn ->
      assert {:ok, _} = connect_with([{"x-council-token", @token}])
    end)
  end

  test "token on: missing header is refused" do
    with_token(@token, fn -> assert :error = connect_with([]) end)
  end

  test "token on: wrong header is refused" do
    with_token(@token, fn ->
      assert :error = connect_with([{"x-council-token", "wrong-token-0123456789"}])
      assert :error = connect_with([{"x-other", @token}])
    end)
  end

  test "token on: a token in the query params is never accepted" do
    with_token(@token, fn ->
      assert :error =
               connect(CouncilHubUiWeb.UserSocket, %{"token" => @token},
                 connect_info: %{x_headers: []}
               )
    end)
  end

  test "token on: the Origin rule still refuses, header or not" do
    conn = build_conn()

    with_token(@token, fn ->
      conn =
        conn
        |> put_req_header("origin", "http://web-page.example")
        |> put_req_header("x-council-token", @token)
        |> get("/socket/websocket?vsn=2.0.0")

      assert conn.status == 403
    end)
  end

  test "token on, over HTTP: a missing header gets 403 from the endpoint" do
    conn = build_conn()

    with_token(@token, fn ->
      conn = get(conn, "/socket/websocket?vsn=2.0.0")
      assert conn.status == 403
    end)
  end
end
