defmodule CouncilHubUiWeb.TrustedListenerTest do
  # Not async: toggles global app env and starts real listeners.
  use CouncilHubUiWeb.ConnCase, async: false

  import Phoenix.LiveViewTest

  alias CouncilHubUi.TrustedListener
  alias CouncilHubUiWeb.Plugs.TrustedEntry

  @token "test-token-0123456789abcdef"
  @outside {172, 17, 0, 1}

  # Test-only wrapper: a real loopback test client would hit the loopback
  # exemption and hide what is being tested, so pretend the peer is outside.
  defmodule Outside do
    def init(opts), do: opts

    def call(conn, next) do
      conn = %{conn | remote_ip: {172, 17, 0, 1}}
      mod = Keyword.fetch!(next, :next)
      mod.call(conn, mod.init(Keyword.get(next, :opts, [])))
    end
  end

  defp with_env(key, value, fun) do
    previous = Application.get_env(:council_hub_ui, key)
    Application.put_env(:council_hub_ui, key, value)

    try do
      fun.()
    after
      case previous do
        nil -> Application.delete_env(:council_hub_ui, key)
        val -> Application.put_env(:council_hub_ui, key, val)
      end
    end
  end

  # Starts a listener on a random loopback port and returns the port.
  defp listen(id, plug) do
    pid =
      start_supervised!(
        Supervisor.child_spec(
          {Bandit, plug: plug, scheme: :http, ip: {127, 0, 0, 1}, port: 0},
          id: id
        )
      )

    {:ok, {_ip, port}} = ThousandIsland.listener_info(pid)
    port
  end

  defp trusted_plug, do: {Outside, next: TrustedEntry}
  defp main_plug, do: {Outside, next: CouncilHubUiWeb.Endpoint}

  defp raw(port, request) do
    {:ok, sock} = :gen_tcp.connect({127, 0, 0, 1}, port, [:binary, active: false])
    :ok = :gen_tcp.send(sock, request)
    reply = recv_all(sock, "")
    :gen_tcp.close(sock)
    reply
  end

  defp recv_all(sock, acc) do
    case :gen_tcp.recv(sock, 0, 3000) do
      {:ok, data} -> recv_all(sock, acc <> data)
      {:error, _} -> acc
    end
  end

  defp http_get(port, path, headers \\ []) do
    extra = Enum.map_join(headers, "", fn {k, v} -> "#{k}: #{v}\r\n" end)
    reply = raw(port, "GET #{path} HTTP/1.1\r\nHost: x\r\nConnection: close\r\n#{extra}\r\n")
    [head | rest] = String.split(reply, "\r\n\r\n", parts: 2)
    {status(head), head, Enum.join(rest)}
  end

  defp status(head) do
    [_, code | _] = String.split(head, " ", parts: 3)
    String.to_integer(code)
  end

  # Websocket upgrade attempt; returns the response status (101 = connected).
  defp ws_status(port, headers \\ []) do
    extra = Enum.map_join(headers, "", fn {k, v} -> "#{k}: #{v}\r\n" end)

    # A fresh handshake nonce per request, as a real client sends.
    nonce = Base.encode64(:crypto.strong_rand_bytes(16))

    request =
      "GET /socket/websocket?vsn=2.0.0 HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\n" <>
        "Connection: Upgrade\r\nSec-WebSocket-Version: 13\r\n" <>
        "Sec-WebSocket-Key: #{nonce}\r\n#{extra}\r\n"

    {:ok, sock} = :gen_tcp.connect({127, 0, 0, 1}, port, [:binary, active: false])
    :ok = :gen_tcp.send(sock, request)
    {:ok, data} = :gen_tcp.recv(sock, 0, 3000)
    :gen_tcp.close(sock)
    status(data)
  end

  describe "validate/2" do
    test "unset or empty is no listener" do
      assert {:ok, nil} = TrustedListener.validate(nil, 4000)
      assert {:ok, nil} = TrustedListener.validate("", 4000)
    end

    test "a port different from the main one is accepted" do
      assert {:ok, 4001} = TrustedListener.validate("4001", 4000)
    end

    test "the main port is refused with a message that never has a token" do
      assert {:error, msg} = TrustedListener.validate("4000", 4000)
      assert msg =~ "COUNCIL_TRUSTED_PORT"
    end

    test "garbage and out-of-range values are refused" do
      for bad <- ["abc", "0", "70000", "40 01", "-1"] do
        assert {:error, _} = TrustedListener.validate(bad, 4000)
      end
    end
  end

  describe "TrustedEntry" do
    test "replaces a client header with the secret and marks the conn" do
      conn =
        Plug.Test.conn(:get, "/")
        |> Plug.Conn.put_req_header("x-council-trusted", "forged-0123456789")
        |> TrustedEntry.call(endpoint: __MODULE__.ConnCaptureEndpoint)

      assert conn.private[:council_trusted] == true
      assert Plug.Conn.get_req_header(conn, "x-council-trusted") == [TrustedListener.secret()]
    end
  end

  describe "main port (token enforced)" do
    test "forged marker header is deleted and gives no bypass", %{conn: conn} do
      with_env(:auth_token, @token, fn ->
        for forged <- [TrustedListener.secret(), "deadbeef0123456789"] do
          resp =
            %{conn | remote_ip: @outside}
            |> put_req_header("x-council-trusted", forged)
            |> get("/status")

          assert resp.status == 401
          assert resp.resp_body == ""
          assert get_resp_header(resp, "set-cookie") == []
          assert get_req_header(resp, "x-council-trusted") == []
        end
      end)
    end
  end

  describe "real listeners" do
    test "trusted port serves pages without credentials, main port does not" do
      with_env(:auth_token, @token, fn ->
        trusted = listen(:trusted_http, trusted_plug())
        main = listen(:main_http, main_plug())

        for path <- ["/", "/status", "/rooms/x/export"] do
          assert {200, _, _} = http_get(trusted, path) |> pad_export(path)
          {401, head, body} = http_get(main, path)
          assert body == ""
          refute head =~ ~r/set-cookie/i
        end
      end)
    end

    test "a forged marker header on the main port gives no bypass" do
      with_env(:auth_token, @token, fn ->
        main = listen(:main_http, main_plug())

        for forged <- [TrustedListener.secret(), "deadbeef0123456789"] do
          assert {401, _, _} = http_get(main, "/status", [{"x-council-trusted", forged}])
        end
      end)
    end

    test "/socket: trusted port connects without a token, main port does not" do
      with_env(:auth_token, @token, fn ->
        trusted = listen(:trusted_ws, trusted_plug())
        main = listen(:main_ws, main_plug())

        assert ws_status(trusted) == 101
        assert ws_status(main) == 403
        # A client cannot present the marker, even with the real secret.
        assert ws_status(main, [{"x-council-trusted", TrustedListener.secret()}]) == 403
        assert ws_status(trusted, [{"x-council-trusted", "forged-0123456789"}]) == 101
        # The token still works on the main port.
        assert ws_status(main, [{"x-council-token", @token}]) == 101
      end)
    end

    test "the Origin rule holds on both listeners" do
      with_env(:auth_token, @token, fn ->
        trusted = listen(:trusted_ws, trusted_plug())
        main = listen(:main_ws, main_plug())
        origin = [{"Origin", "http://web-page.example"}]

        assert ws_status(trusted, origin) == 403
        assert ws_status(main, origin ++ [{"x-council-token", @token}]) == 403
      end)
    end
  end

  describe "reporting" do
    test "/status line and cluster output show trusted_listener", %{conn: conn} do
      with_env(:trusted_port, 4001, fn ->
        {:ok, _view, html} = live(conn, "/status")
        assert html =~ "Trusted local listener: on (port 4001; publish it on loopback only)"
        body = conn |> get("/api/internal/cluster/nodes") |> json_response(200)
        assert body["trusted_listener"] == true
      end)

      with_env(:trusted_port, nil, fn ->
        {:ok, _view, html} = live(conn, "/status")
        assert html =~ "Trusted local listener: off"
        body = conn |> get("/api/internal/cluster/nodes") |> json_response(200)
        assert body["trusted_listener"] == false
      end)
    end
  end

  # The export route 404s for an unknown room once past auth; reaching it
  # (not a 401) is what this test cares about.
  defp pad_export({404, h, b}, "/rooms/x/export"), do: {200, h, b}
  defp pad_export(other, _), do: other

  defmodule ConnCaptureEndpoint do
    def init(opts), do: opts
    def call(conn, _opts), do: conn
  end
end
