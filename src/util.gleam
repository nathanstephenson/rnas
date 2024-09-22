import gleam/bytes_builder
import gleam/http/response.{type Response}
import gleam/io
import gleam/json
import mist

pub fn println(it, str) {
  io.println(str)
  it
}

pub fn response(data: json.Json) -> Response(mist.ResponseData) {
  let res_data =
    data
    |> json.to_string
    |> bytes_builder.from_string
    |> mist.Bytes
  response.new(200)
  |> response.set_body(res_data)
}
