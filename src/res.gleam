import fs.{type FSEntry}
import gleam/json.{type Json}
import gleam/list

pub fn dir(input: List(FSEntry)) -> Json {
  json.object([
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
  ])
}
