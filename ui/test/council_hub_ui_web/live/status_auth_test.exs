defmodule CouncilHubUiWeb.StatusAuthTest do
  # Not async: toggles the global :auth_token application env.
  use CouncilHubUiWeb.ConnCase, async: false

  import Phoenix.LiveViewTest

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

  test "shows Authentication: on", %{conn: conn} do
    with_token("test-token-0123456789abcdef", fn ->
      {:ok, _view, html} = live(conn, "/status")
      assert html =~ "Authentication: on"
      refute html =~ "test-token-0123456789abcdef"
    end)
  end

  test "shows the off warning", %{conn: conn} do
    with_token(nil, fn ->
      {:ok, _view, html} = live(conn, "/status")

      assert html =~
               "Authentication: off — anyone who can reach this port can read the dashboard"
    end)
  end
end
