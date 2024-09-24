import dot_env
import fs
import gleam/bool
import gleam/bytes_builder
import gleam/erlang/process
import gleam/http/request.{type Request}
import gleam/http/response.{type Response}
import gleam/io
import gleam/iterator
import gleam/json
import gleam/list
import gleam/option.{None, Some}
import gleam/otp/actor
import gleam/result
import gleam/string
import mist
import util

pub fn main() {
  io.println("Hello from rnas!")

  io.println("Loading environment variables from ./.env")
  dot_env.new()
  |> dot_env.set_path("./.env")
  |> dot_env.load

  let selector = process.new_selector()
  let state = Nil

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
        path -> string.join(path, "/") |> handle_req
      }
    }
    |> mist.new
    |> mist.port(3000)
    |> mist.start_http

  process.sleep_forever()
}

fn handle_req(path: String) -> Response(mist.ResponseData) {
  let is_file = string.split(path, ".") |> list.length > 1

  let not_found =
    response.new(404) |> response.set_body(mist.Bytes(bytes_builder.new()))

  io.println(string.append("Handling request for: ", path))
  io.println(string.append("should be file? ", is_file |> bool.to_string))

  case is_file {
    True -> not_found
    False -> {
      case fs.get_dir(path) {
        Error(err) -> {
          io.println(string.append("Error: ", err))
          not_found
        }
        Ok(dir) ->
          json.object([
            #("count", list.length(dir) |> json.int),
            #(
              "files",
              json.array(
                dir
                  |> list.fold([], fn(acc, entry) {
                    case entry {
                      fs.File(name) -> [name, ..acc]
                      _ -> acc
                    }
                  }),
                json.string,
              ),
            ),
            #(
              "dirs",
              json.array(
                dir
                  |> list.fold([], fn(acc, entry) {
                    case entry {
                      fs.Directory(name) -> [name, ..acc]
                      _ -> acc
                    }
                  }),
                json.string,
              ),
            ),
          ])
          |> util.response
      }
    }
  }
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

fn guess_content_type(file_path: String) -> String {
  string.split(file_path, ".")
  |> list.last
  |> result.map(fn(ext) {
    case ext {
      "html" -> "text/html"
      "css" -> "text/css"
      "js" -> "application/javascript"
      "json" -> "application/json"
      "png" -> "image/png"
      "jpg" -> "image/jpeg"
      "jpeg" -> "image/jpeg"
      "gif" -> "image/gif"
      "svg" -> "image/svg+xml"
      "ico" -> "image/x-icon"
      "txt" -> "text/plain"
      _ -> "application/octet-stream"
    }
  })
  |> result.lazy_unwrap(fn() { "text/plain" })
}
