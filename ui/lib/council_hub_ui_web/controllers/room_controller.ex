defmodule CouncilHubUiWeb.RoomController do
  use CouncilHubUiWeb, :controller

  alias CouncilHubUi.Council

  def export(conn, %{"room_id" => room_id}) do
    case Council.get_room(room_id) do
      nil ->
        conn
        |> put_status(:not_found)
        |> text("Room '#{room_id}' not found.")

      room ->
        messages = Council.list_messages_for_room(room_id)
        transcript = Council.format_transcript(room, messages)

        conn
        |> put_resp_content_type("text/markdown")
        |> put_resp_header("content-disposition", ~s(attachment; filename="#{room_id}.md"))
        |> send_resp(200, transcript)
    end
  end
end
