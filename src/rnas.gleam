import gleam/bytes_builder
import gleam/erlang/process
import gleam/http/request.{type Request}
import gleam/http/response.{type Response}
import gleam/io
import gleam/iterator
import gleam/option.{None, Some}
import gleam/otp/actor
import gleam/result
import gleam/string
import mist

pub fn main() {
  io.println("Hello from rnas!")

  let selector = process.new_selector()
  let state = Nil

  let not_found =
    response.new(404) |> response.set_body(mist.Bytes(bytes_builder.new()))

  let assert Ok(_) =
    fn(req: Request(mist.Connection)) -> Response(mist.ResponseData) {
      case request.path_segments(req) {
        ["ws"] ->
          mist.websocket(
            request: req,
            on_init: fn(_conn) { #(state, Some(selector)) },
            on_close: fn(_conn) { io.println("Websocket closed") },
            handler: handle_ws_message,
          )
        ["echo"] -> echo_body(req)
        ["chunk"] -> serve_chunk(req)
        ["file", ..rest] -> serve_file(req, rest)
        ["form"] -> handle_form(req)
        _ -> not_found
      }
    }
    |> mist.new
    |> mist.port(3000)
    |> mist.start_http

  process.sleep_forever()
}

pub type MyMessage {
  Broadcast(String)
}

fn handle_ws_message(state, conn, message) {
  case message {
    mist.Text("ping") -> {
      let assert Ok(_) = mist.send_text_frame(conn, "pong")
      actor.continue(state)
    }
    mist.Text(_) | mist.Binary(_) -> {
      actor.continue(state)
    }
    mist.Custom(Broadcast(text)) -> {
      let assert Ok(_) = mist.send_text_frame(conn, text)
      actor.continue(state)
    }
    mist.Closed | mist.Shutdown -> actor.Stop(process.Normal)
  }
}

fn echo_body(req: Request(mist.Connection)) -> Response(mist.ResponseData) {
  let content_type =
    req
    |> request.get_header("content-type")
    |> result.unwrap("text/plain")

  mist.read_body(req, 1024 * 1024 * 10)
  |> result.map(fn(req) {
    response.new(200)
    |> response.set_body(mist.Bytes(bytes_builder.from_bit_array(req.body)))
    |> response.set_header("content-type", content_type)
  })
  |> result.lazy_unwrap(fn() {
    response.new(400)
    |> response.set_body(mist.Bytes(bytes_builder.new()))
  })
}

fn serve_chunk(_req: Request(mist.Connection)) -> Response(mist.ResponseData) {
  let iter =
    ["1", "2", "3"]
    |> iterator.from_list
    |> iterator.map(bytes_builder.from_string)

  response.new(200)
  |> response.set_body(mist.Chunked(iter))
  |> response.set_header("content-type", "text/plain")
}

fn serve_file(
  _req: Request(mist.Connection),
  path: List(String),
) -> Response(mist.ResponseData) {
  let file_path = string.join(path, "/")

  mist.send_file(file_path, offset: 0, limit: None)
  |> result.map(fn(file) {
    let content_type = guess_content_type(file_path)
    response.new(200)
    |> response.prepend_header("content-type", content_type)
    |> response.set_body(file)
  })
  |> result.lazy_unwrap(fn() {
    response.new(404)
    |> response.set_body(mist.Bytes(bytes_builder.new()))
  })
}

fn handle_form(req: Request(mist.Connection)) -> Response(mist.ResponseData) {
  let _req = mist.read_body(req, 1024 * 1024 * 10)
  response.new(200)
  |> response.set_body(mist.Bytes(bytes_builder.from_string("Form received")))
}

fn guess_content_type(_file_path: String) -> String {
  "text/plain"
}
