defmodule CouncilHubUiWeb.CouncilComponents do
  @moduledoc """
  Thin facade re-exporting all function components from the split sub-modules.
  Kept for backward compatibility with tests and any external callers.

  New code should import the specific module directly:
    - CouncilHubUiWeb.RoomComponents
    - CouncilHubUiWeb.MessageComponents
    - CouncilHubUiWeb.PanelComponents
  """

  defdelegate room_card(assigns), to: CouncilHubUiWeb.RoomComponents
  defdelegate room_header(assigns), to: CouncilHubUiWeb.RoomComponents
  defdelegate message_bubble(assigns), to: CouncilHubUiWeb.MessageComponents
  defdelegate summary_block(assigns), to: CouncilHubUiWeb.MessageComponents
  defdelegate mentions_panel(assigns), to: CouncilHubUiWeb.PanelComponents
  defdelegate archive_list(assigns), to: CouncilHubUiWeb.PanelComponents
  defdelegate archive_modal(assigns), to: CouncilHubUiWeb.PanelComponents
end
