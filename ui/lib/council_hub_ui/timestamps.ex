defmodule CouncilHubUi.Timestamps do
  @moduledoc """
  Lenient timestamp parsing shared by every read-side query module
  (CouncilMessages, CouncilNotebook, CouncilDigest). Accepts, in order:

    * full ISO 8601 datetime — `"2026-07-01T10:00:00"` or `"2026-07-01 10:00:00"`
    * minute precision — `"2026-07-01 10:00"` (seconds padded to `:00`)
    * date only — `"2026-07-01"` (padded with the given time of day)

  Anything unparseable returns nil so a bad filter is dropped instead of an
  invalid cast reaching Ecto (Ecto.Query.CastError crashes the LiveView).

  `parse_since/1` pads a date-only value to start of day; `parse_until/1` pads
  to end of day so an `until` date is inclusive of that whole day.
  """

  @doc "Parse a `since` bound: date-only values become start of day (00:00:00)."
  def parse_since(value), do: parse(value, ~T[00:00:00])

  @doc "Parse an `until` bound: date-only values become end of day (23:59:59), inclusive."
  def parse_until(value), do: parse(value, ~T[23:59:59])

  @doc "Parse a timestamp, padding a date-only value with `default_time`."
  def parse(value, default_time \\ ~T[00:00:00])
  def parse(nil, _default_time), do: nil
  def parse("", _default_time), do: nil
  def parse(%NaiveDateTime{} = ts, _default_time), do: ts

  def parse(str, default_time) when is_binary(str) do
    str = String.trim(str)

    full_datetime(str) || minute_precision(str) || date_only(str, default_time)
  end

  defp full_datetime(str) do
    case NaiveDateTime.from_iso8601(str) do
      {:ok, ts} -> ts
      _ -> nil
    end
  end

  # "2026-07-01 10:00" / "2026-07-01T10:00" — from_iso8601 requires seconds, but
  # minute precision is a natural hand-typed form (and Ecto's cast accepted it).
  defp minute_precision(str) do
    if Regex.match?(~r/^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}$/, str) do
      full_datetime(str <> ":00")
    end
  end

  defp date_only(str, default_time) do
    case Date.from_iso8601(str) do
      {:ok, date} -> NaiveDateTime.new!(date, default_time)
      _ -> nil
    end
  end
end
