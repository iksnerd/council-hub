defmodule CouncilHubUi.ParamsTest do
  use ExUnit.Case, async: true

  doctest CouncilHubUi.Params

  alias CouncilHubUi.Params

  test "integer below min falls back to default, not clamped up" do
    assert Params.clamp_int(0, 50, max: 100) == 50
    assert Params.clamp_int(-3, 50, max: 100) == 50
  end

  test "min: 0 accepts zero (offset semantics)" do
    assert Params.clamp_int(0, 0, min: 0) == 0
    assert Params.clamp_int("-5", 0, min: 0) == 0
  end

  test "unparseable and non-scalar values fall back to default" do
    assert Params.clamp_int("abc", 20, max: 100) == 20
    assert Params.clamp_int(%{}, 20, max: 100) == 20
  end
end
