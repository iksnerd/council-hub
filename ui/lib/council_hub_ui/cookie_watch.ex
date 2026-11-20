defmodule CouncilHubUi.CookieWatch do
  @moduledoc """
  A `:logger` handler that notices when the BEAM rejects an inbound
  distribution connection for a bad cookie.

  OTP's `dist_util` logs `Connection attempt from node ~w rejected. Invalid
  challenge reply.` on the node that *accepts* a connection whose dialer used a
  different cookie. That is the only place the mismatch is observable: the
  dialer just sees `:pang`, which looks the same as a closed port or a wrong
  name. So this is inbound-only. If only this node dials the peer, nothing is
  logged here and the finding stays `undialable`.

  Only the dialing node's name is forwarded to `ClusterManager`; the log event
  carries no cookie and nothing else is kept.
  """

  @handler_id :council_cookie_watch

  @doc "Install the handler. Idempotent."
  def install do
    case :logger.add_handler(@handler_id, __MODULE__, %{}) do
      :ok -> :ok
      {:error, {:already_exist, _}} -> :ok
      other -> other
    end
  end

  @doc "Extract the dialing node from a bad-cookie rejection event, else `:ignore`."
  def rejected_node(%{msg: {:report, %{format: format, args: [node]}}}),
    do: match_rejection(format, node)

  def rejected_node(%{msg: {format, [node]}}) when is_list(format),
    do: match_rejection(format, node)

  def rejected_node(_), do: :ignore

  defp match_rejection(format, node) do
    if rejection_format?(format) and is_atom(node), do: {:ok, Atom.to_string(node)}, else: :ignore
  end

  defp rejection_format?(format) do
    format |> IO.chardata_to_string() |> String.contains?("Invalid challenge reply")
  rescue
    _ -> false
  end

  ## :logger handler callback

  @doc false
  def log(event, _config) do
    with {:ok, node} <- rejected_node(event),
         pid when is_pid(pid) <- Process.whereis(CouncilHubUi.ClusterManager) do
      send(pid, {:cookie_rejected, node})
    end

    :ok
  rescue
    _ -> :ok
  end
end
