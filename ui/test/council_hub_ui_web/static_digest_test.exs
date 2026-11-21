defmodule CouncilHubUiWeb.StaticDigestTest do
  # In production Phoenix rewrites /favicon.ico to /favicon-<hash>.ico through
  # cache_static_manifest. Plug.Static's `only` matches exact first segments, so
  # without `only_matching` the hashed names 404 (the browser console showed 404s
  # for the favicon and the web manifest). These tests drop a hashed file into the
  # static folder and request it through the endpoint.
  use CouncilHubUiWeb.ConnCase, async: true

  defp static_dir, do: Application.app_dir(:council_hub_ui, "priv/static")

  defp with_static_file(name, body, fun) do
    path = Path.join(static_dir(), name)
    File.write!(path, body)

    try do
      fun.()
    after
      File.rm(path)
    end
  end

  defp unique, do: "t#{System.unique_integer([:positive])}0123456789abcdef"

  for {prefix, ext} <- [
        {"favicon", "ico"},
        {"favicon", "svg"},
        {"apple-touch-icon", "png"},
        {"site", "webmanifest"}
      ] do
    test "a digested #{prefix}.#{ext} is served", %{conn: conn} do
      name = "#{unquote(prefix)}-#{unique()}.#{unquote(ext)}"

      with_static_file(name, "bytes-of-#{name}", fn ->
        conn = get(conn, "/" <> name)
        assert conn.status == 200
        assert conn.resp_body == "bytes-of-#{name}"
      end)
    end
  end

  test "the unhashed favicon still works", %{conn: conn} do
    assert get(conn, "/favicon.ico").status == 200
  end

  test "the prefix match does not open up unrelated names", %{conn: conn} do
    name = "robots-#{unique()}.txt"

    with_static_file(name, "should not be served", fn ->
      assert get(conn, "/" <> name).status == 404
    end)
  end
end
