import fs.{type FSEntry}
import gleam/bit_array
import gleam/json.{type Json}
import gleam/list

pub fn dir(input: List(FSEntry)) -> Json {
  [
    #("type", json.string("directory")),
    #("count", list.length(input) |> json.int),
    #(
      "files",
      json.array(
        input
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
        input
          |> list.fold([], fn(acc, entry) {
            case entry {
              fs.Directory(name) -> [name, ..acc]
              _ -> acc
            }
          }),
        json.string,
      ),
    ),
  ]
  |> json.object
}

pub fn file(input: BitArray) -> Json {
  [
    #("type", json.string("file")),
    #("bytes", input |> bit_array.base64_encode(True) |> json.string),
    #("total_len", input |> bit_array.byte_size |> json.int),
  ]
  |> json.object
}
