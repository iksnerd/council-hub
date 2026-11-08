defmodule CouncilHubUiWeb.SettingsLiveTest do
  # async: false — these tests mutate the global COUNCIL_CLUSTER_ADMIN_TOKEN env var.
  use CouncilHubUiWeb.ConnCase, async: false

  import Phoenix.LiveViewTest

  @token "test-admin-token"

  setup do
    System.put_env("COUNCIL_CLUSTER_ADMIN_TOKEN", @token)
    on_exit(fn -> System.delete_env("COUNCIL_CLUSTER_ADMIN_TOKEN") end)
    :ok
  end

  defp admin_conn(conn), do: Plug.Test.init_test_session(conn, %{cluster_admin: true})

  describe "gating" do
    test "404 when no admin token is configured on the server", %{conn: conn} do
      System.delete_env("COUNCIL_CLUSTER_ADMIN_TOKEN")
      conn = get(conn, "/settings")
      assert conn.status == 404
    end

    test "403 without a session and without the token param", %{conn: conn} do
      conn = get(conn, "/settings")
      assert conn.status == 403
    end

    test "correct ?token= sets the session and redirects to the clean URL", %{conn: conn} do
      conn = get(conn, "/settings?token=#{@token}")
      assert redirected_to(conn) == "/settings"
      assert get_session(conn, :cluster_admin) == true
    end

    test "wrong ?token= is rejected", %{conn: conn} do
      conn = get(conn, "/settings?token=nope")
      assert conn.status == 403
    end
  end

  describe "mount (authenticated)" do
    test "renders this node's identity and the connect form", %{conn: conn} do
      {:ok, _view, html} = live(admin_conn(conn), "/settings")

      assert html =~ "Cluster Settings"
      assert html =~ to_string(Node.self())
      assert html =~ "Connect a peer"
      assert html =~ "council_hub@100.x.y.z"
    end
  end

  describe "connect event" do
    test "shows an error flash for a malformed node name", %{conn: conn} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")

      html =
        view
        |> form("form[phx-submit=connect]", %{"node" => "garbage"})
        |> render_submit()

      assert html =~ "invalid node name"
    end
  end

  describe "tailscale card" do
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

    test "off: a Sign in button, owner-only default, no sign-in link", %{conn: conn} do
      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      assert html =~ "Sign in with Tailscale"
      assert html =~ "Only my Tailscale account"
      refute html =~ "Continue in your browser"
      refute html =~ "unavailable"
    end

    test "needs-login: the link opens only when it is an https URL", %{conn: conn, agent: agent} do
      reply = fn url ->
        {200,
         Jason.encode!(%{
           "state" => "needs-login",
           "auth_url" => url,
           "config" => %{"enabled" => true}
         })}
      end

      Agent.update(agent, &Map.put(&1, :reply, reply.("https://login.tailscale.com/a/abc")))
      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      assert html =~ "https://login.tailscale.com/a/abc"
      assert html =~ "Continue in your browser"
      assert html =~ "noopener noreferrer"
      assert html =~ "Cancel sign-in"
      refute html =~ "Sign in with Tailscale"

      Agent.update(agent, &Map.put(&1, :reply, reply.("javascript:alert(1)")))
      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      refute html =~ "javascript:alert"
      assert html =~ "Preparing the Tailscale sign-in"
    end

    test "running: shows the URL, the connection command and Sign out", %{
      conn: conn,
      agent: agent
    } do
      Agent.update(
        agent,
        &Map.put(
          &1,
          :reply,
          {200,
           Jason.encode!(%{
             "state" => "running",
             "url" => "https://hub.tail1.ts.net",
             "owner" => "me@example.com",
             "config" => %{"enabled" => true, "hostname" => "hub", "access" => "owner"}
           })}
        )
      )

      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      assert html =~ "https://hub.tail1.ts.net"
      assert html =~ "claude mcp add --transport http council-hub https://hub.tail1.ts.net/mcp"
      assert html =~ "Sign out"
      refute html =~ "Sign in with Tailscale"
    end

    test "Sign in sends enabled true with the form's defaults", %{conn: conn, agent: agent} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")

      view
      |> form("form[phx-submit=ts_apply]", %{
        "hostname" => "hub-two",
        "access" => "members",
        "tags" => "tag:council, tag:ci",
        "auth_key" => "tskey-auth-xyz"
      })
      |> render_submit(%{"intent" => "signin"})

      {"POST", body} = Agent.get(agent, & &1.last)
      sent = Jason.decode!(body)
      assert sent["enabled"] == true
      assert sent["hostname"] == "hub-two"
      assert sent["access"] == "members"
      assert sent["tags"] == ["tag:council", "tag:ci"]
      assert sent["auth_key"] == "tskey-auth-xyz"
    end

    test "Save settings while off does not turn Tailscale on", %{conn: conn, agent: agent} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")

      view
      |> form("form[phx-submit=ts_apply]", %{"hostname" => "hub-two"})
      |> render_submit(%{"intent" => "save"})

      {"POST", body} = Agent.get(agent, & &1.last)
      assert Jason.decode!(body)["enabled"] == false
    end

    test "Cancel sends a bare enabled false", %{conn: conn, agent: agent} do
      Agent.update(
        agent,
        &Map.put(
          &1,
          :reply,
          {200,
           Jason.encode!(%{
             "state" => "needs-login",
             "auth_url" => "https://login.tailscale.com/a/abc",
             "config" => %{"enabled" => true}
           })}
        )
      )

      {:ok, view, _html} = live(admin_conn(conn), "/settings")
      Agent.update(agent, &Map.put(&1, :reply, {200, ~s({"state":"off","config":{}})}))
      render_click(view, "ts_cancel", %{})

      {"POST", body} = Agent.get(agent, & &1.last)
      assert Jason.decode!(body) == %{"enabled" => false}
    end

    test "Sign out sends the logout action", %{conn: conn, agent: agent} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")
      render_click(view, "ts_signout", %{})

      {"POST", body} = Agent.get(agent, & &1.last)
      assert Jason.decode!(body) == %{"action" => "logout"}
    end

    defp ts_reply(agent, map),
      do: Agent.update(agent, &Map.put(&1, :reply, {200, Jason.encode!(map)}))

    defp needs_login(url),
      do: %{"state" => "needs-login", "auth_url" => url, "config" => %{"enabled" => true}}

    test "after Sign in, the link is pushed once so the browser can open it", %{
      conn: conn,
      agent: agent
    } do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")

      ts_reply(agent, %{"state" => "starting", "config" => %{"enabled" => true}})

      view
      |> form("form[phx-submit=ts_apply]", %{})
      |> render_submit(%{"intent" => "signin"})

      refute_push_event(view, "ts_auth_url", _, 50)

      ts_reply(agent, needs_login("https://login.tailscale.com/a/abc"))
      send(view.pid, :ts_poll)
      assert_push_event(view, "ts_auth_url", %{url: "https://login.tailscale.com/a/abc"})

      # Polled again with the same link: the browser must not be sent it twice.
      send(view.pid, :ts_poll)
      _ = render(view)
      refute_push_event(view, "ts_auth_url", _, 100)
    end

    test "the link is pushed at once when it is already there at Sign in", %{
      conn: conn,
      agent: agent
    } do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")
      ts_reply(agent, needs_login("https://login.tailscale.com/a/now"))

      view
      |> form("form[phx-submit=ts_apply]", %{})
      |> render_submit(%{"intent" => "signin"})

      assert_push_event(view, "ts_auth_url", %{url: "https://login.tailscale.com/a/now"})
    end

    test "a page that loads already waiting for sign-in does not open a window", %{
      conn: conn,
      agent: agent
    } do
      ts_reply(agent, needs_login("https://login.tailscale.com/a/abc"))
      {:ok, view, html} = live(admin_conn(conn), "/settings")
      assert html =~ "Continue in your browser"

      send(view.pid, :refresh)
      send(view.pid, :ts_poll)
      _ = render(view)
      refute_push_event(view, "ts_auth_url", _, 100)
    end

    test "only an https link is ever pushed", %{conn: conn, agent: agent} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")
      ts_reply(agent, needs_login("javascript:alert(1)"))

      view
      |> form("form[phx-submit=ts_apply]", %{})
      |> render_submit(%{"intent" => "signin"})

      send(view.pid, :ts_poll)
      _ = render(view)
      refute_push_event(view, "ts_auth_url", _, 100)
    end

    test "Cancel stops waiting, so a later link is not opened", %{conn: conn, agent: agent} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")
      ts_reply(agent, %{"state" => "starting", "config" => %{"enabled" => true}})

      view |> form("form[phx-submit=ts_apply]", %{}) |> render_submit(%{"intent" => "signin"})

      ts_reply(agent, %{"state" => "off", "config" => %{"enabled" => false}})
      render_click(view, "ts_cancel", %{})

      ts_reply(agent, needs_login("https://login.tailscale.com/a/late"))
      send(view.pid, :ts_poll)
      _ = render(view)
      refute_push_event(view, "ts_auth_url", _, 100)
    end

    test "the Sign in button is marked so the browser opens the window on the click", %{
      conn: conn
    } do
      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      assert html =~ "data-ts-signin"
    end

    test "Copy link carries the sign-in link", %{conn: conn, agent: agent} do
      ts_reply(agent, needs_login("https://login.tailscale.com/a/abc"))
      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      assert html =~ ~s(data-copy="https://login.tailscale.com/a/abc")
      assert html =~ "Copy link"
    end

    test "running: Copy carries the connection command", %{conn: conn, agent: agent} do
      ts_reply(agent, %{
        "state" => "running",
        "url" => "https://hub.tail1.ts.net",
        "owner" => "me@example.com",
        "config" => %{"enabled" => true}
      })

      {:ok, _view, html} = live(admin_conn(conn), "/settings")

      assert html =~
               ~s(data-copy="claude mcp add --transport http council-hub https://hub.tail1.ts.net/mcp")
    end

    test "a rejected config shows the Go server's message", %{conn: conn, agent: agent} do
      {:ok, view, _html} = live(admin_conn(conn), "/settings")

      Agent.update(
        agent,
        &Map.put(&1, :reply, {400, "hostname must be 1-63 lowercase letters\n"})
      )

      html =
        view
        |> form("form[phx-submit=ts_apply]", %{"hostname" => "NO"})
        |> render_submit(%{"intent" => "signin"})

      assert html =~ "hostname must be 1-63 lowercase letters"
    end

    test "says so when the Go server does not answer", %{conn: conn} do
      Application.put_env(:council_hub_ui, :mcp_server_url, "http://127.0.0.1:19999/mcp")
      {:ok, _view, html} = live(admin_conn(conn), "/settings")
      assert html =~ "unavailable"
      assert html =~ "did not answer"
    end
  end
end
