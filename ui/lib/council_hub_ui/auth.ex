defmodule CouncilHubUi.Auth do
  @moduledoc """
  The shared token behind `COUNCIL_AUTH_TOKEN`.

  Unset or empty means authentication is off and every surface behaves as it
  always did. When set, the dashboard (`Plugs.RequireDashboardAuth`) and the
  presence socket (`UserSocket`) require it. `config/runtime.exs` runs the
  value through `validate/1` at boot, so a token that is too short stops the
  app from starting rather than silently protecting nothing.

  The token is read from application config (`:auth_token`), never from the
  environment at request time, and no function here logs it.
  """

  @min_length 16

  @doc "Minimum token length, in characters."
  def min_length, do: @min_length

  @doc """
  Validates the raw `COUNCIL_AUTH_TOKEN` value for boot.

  Returns `{:ok, nil}` when auth is off (unset or empty), `{:ok, token}` when
  usable, or `{:error, message}` when too short. The message names the variable
  and the minimum, never the value.
  """
  @spec validate(String.t() | nil) :: {:ok, String.t() | nil} | {:error, String.t()}
  def validate(nil), do: {:ok, nil}
  def validate(""), do: {:ok, nil}

  def validate(token) when is_binary(token) do
    if String.length(token) >= @min_length do
      {:ok, token}
    else
      {:error,
       "COUNCIL_AUTH_TOKEN is set but shorter than the minimum of #{@min_length} characters. " <>
         "Refusing to start with a weak token. Generate one with: openssl rand -hex 32"}
    end
  end

  @doc "The configured token, or nil when auth is off."
  def token do
    case Application.get_env(:council_hub_ui, :auth_token) do
      t when is_binary(t) and t != "" -> t
      _ -> nil
    end
  end

  @doc "True when a token is configured."
  def enabled?, do: token() != nil

  @doc """
  Constant-time check of a candidate against the configured token. Both sides
  are hashed with SHA-256 first, so the compared values always have equal
  length. False when auth is off or the candidate is not a binary.
  """
  @spec valid?(term()) :: boolean()
  def valid?(candidate) when is_binary(candidate) do
    case token() do
      nil -> false
      expected -> Plug.Crypto.secure_compare(digest(candidate), digest(expected))
    end
  end

  def valid?(_), do: false

  defp digest(value), do: :crypto.hash(:sha256, value)

  @doc "The `/status` line about the token, as a `{state, text}` pair."
  def status_line(true), do: {:on, "Authentication: on"}

  def status_line(false),
    do: {:off, "Authentication: off — anyone who can reach this port can read the dashboard"}

  @doc """
  The `/status` warning when the token is on but the cluster cookie is the public
  default and clustering is in use, else nil.
  """
  def cookie_warning(auth_enabled?, cookie_default?, seeds, peers) do
    clustered? = seeds not in [nil, ""] or peers != []

    if auth_enabled? and cookie_default? and clustered? do
      "The token does not protect the cluster ports or /api/internal/* while the cluster " <>
        "cookie is the default; set RELEASE_COOKIE to the same non-default value on every node."
    end
  end
end
