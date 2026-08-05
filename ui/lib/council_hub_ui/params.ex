defmodule CouncilHubUi.Params do
  @moduledoc """
  Shared "parse a limit/offset param, clamp it, fall back to a default"
  helper — every read-side query module and the cluster controller
  reimplemented this with slightly different (and in one case inconsistent)
  bounds-checking. `clamp_int/3` is the single source of truth.
  """

  @doc """
  Coerces `value` (an integer, a numeric string, or nil/"") to an integer
  bounded by `:min` (default 1) and `:max` (default unbounded).

  A value at or above `:min` is kept, clamped down to `:max` if it exceeds it.
  `nil`, `""`, a value below `:min`, or anything unparseable falls back to
  `default` — not clamped up to `:min` — matching the existing "reject a
  non-positive limit rather than silently rewrite it" convention.

      iex> CouncilHubUi.Params.clamp_int("20", 10, max: 100)
      20
      iex> CouncilHubUi.Params.clamp_int("150", 10, max: 100)
      100
      iex> CouncilHubUi.Params.clamp_int("0", 10, max: 100)
      10
      iex> CouncilHubUi.Params.clamp_int(nil, 10, max: 100)
      10
      iex> CouncilHubUi.Params.clamp_int("-5", 0, min: 0)
      0
  """
  def clamp_int(value, default, opts \\ [])
  def clamp_int(nil, default, _opts), do: default
  def clamp_int("", default, _opts), do: default

  def clamp_int(val, default, opts) when is_integer(val) do
    min = Keyword.get(opts, :min, 1)
    max = Keyword.get(opts, :max)

    cond do
      val < min -> default
      is_integer(max) and val > max -> max
      true -> val
    end
  end

  def clamp_int(val, default, opts) when is_binary(val) do
    case Integer.parse(val) do
      {n, _} -> clamp_int(n, default, opts)
      :error -> default
    end
  end

  def clamp_int(_val, default, _opts), do: default
end
