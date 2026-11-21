defmodule CouncilHubUiWeb.TrustedScrub do
  @moduledoc """
  Makes the endpoint delete any client-supplied `x-council-trusted` header
  before anything reads it.

  A plug inside the endpoint is too late: Phoenix dispatches `/socket` and
  `/live` in its own first plug, ahead of the endpoint's. So this wraps the
  endpoint's whole `call/2` instead, through a `@before_compile` registered after
  Phoenix's own (callbacks run in registration order, so this one sees Phoenix's
  `call/2` and wraps it with `super`).

  A request that came through the trusted listener already carries the marker in
  `conn.private[:council_trusted]` (set by `Plugs.TrustedEntry`, which also
  added the header with the per-boot secret); those are left alone.
  """

  defmacro __before_compile__(_env) do
    quote do
      defoverridable call: 2

      def call(conn, opts) do
        super(CouncilHubUiWeb.TrustedScrub.scrub(conn), opts)
      end
    end
  end

  @doc false
  def scrub(%Plug.Conn{private: %{council_trusted: true}} = conn), do: conn

  def scrub(conn),
    do: Plug.Conn.delete_req_header(conn, CouncilHubUi.TrustedListener.header())
end
