defmodule CouncilHubUi.TimestampsTest do
  use ExUnit.Case, async: true

  alias CouncilHubUi.Timestamps

  describe "full ISO datetime" do
    test "parses T-separated datetime" do
      assert Timestamps.parse_since("2026-07-01T10:30:15") == ~N[2026-07-01 10:30:15]
    end

    test "parses space-separated datetime" do
      assert Timestamps.parse_since("2026-07-01 10:30:15") == ~N[2026-07-01 10:30:15]
    end

    test "passes a NaiveDateTime struct through unchanged" do
      ts = ~N[2026-07-01 10:30:15]
      assert Timestamps.parse_since(ts) == ts
      assert Timestamps.parse_until(ts) == ts
    end
  end

  describe "minute precision" do
    test "pads seconds for a space-separated value" do
      assert Timestamps.parse_since("2026-07-01 10:00") == ~N[2026-07-01 10:00:00]
    end

    test "pads seconds for a T-separated value" do
      assert Timestamps.parse_until("2026-07-01T10:00") == ~N[2026-07-01 10:00:00]
    end
  end

  describe "date only" do
    test "since pads to start of day" do
      assert Timestamps.parse_since("2026-07-01") == ~N[2026-07-01 00:00:00]
    end

    test "until pads to end of day (inclusive of the whole day)" do
      assert Timestamps.parse_until("2026-07-01") == ~N[2026-07-01 23:59:59]
    end
  end

  describe "garbage and empty input" do
    test "nil and empty string yield nil" do
      assert Timestamps.parse_since(nil) == nil
      assert Timestamps.parse_since("") == nil
      assert Timestamps.parse_until(nil) == nil
      assert Timestamps.parse_until("") == nil
    end

    test "unparseable strings yield nil" do
      assert Timestamps.parse_since("not a date") == nil
      assert Timestamps.parse_since("2026-13-99") == nil
      assert Timestamps.parse_until("2026-07-01 25:99") == nil
    end
  end
end
