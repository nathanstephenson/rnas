import dot_env/env
import file_streams/file_stream
import fswalk
import gcalc
import gleam/float
import gleam/int
import gleam/io
import gleam/iterator
import gleam/list
import gleam/result
import gleam/string
import util

pub fn read(path: String) -> Result(BitArray, String) {
  get_valid_path(path)
  |> result.try(fn(path) {
    let max_file_size = int.to_float(env.get_int_or("MAX_FILE_SIZE_MB", 1024))
    file_stream.open_read(path)
    |> result.map(fn(stream) {
      max_file_size *. gcalc.pow(1024.0, 3.0)
      |> float.floor
      |> float.to_string
      |> int.parse
      |> result.map(fn(byte_len) {
        case file_stream.read_bytes(stream, byte_len) {
          Ok(bytes) -> Ok(bytes)
          _ -> Error("Failed to read file")
        }
      })
      |> result.lazy_unwrap(fn() {
        Error("Failed to read file - invalid byte length")
      })
    })
    |> result.lazy_unwrap(fn() { Error("Failed to open file") })
  })
}

pub type FSEntry {
  File(String)
  Directory(String)
}

pub fn get_dir(path: String) -> Result(List(FSEntry), String) {
  let roots = get_root_paths()
  case path {
    "" -> Ok(list.map(roots, fn(p) { Directory(p.name) }))
    _ ->
      get_valid_path(path)
      |> result.try(fn(path) {
        fswalk.builder()
        |> fswalk.with_path(path)
        |> util.println(path)
        |> fswalk.walk
        |> iterator.try_fold([], fn(acc, it) {
          case it {
            Ok(entry) -> {
              let path_to_remove = string.append(path, "/")
              let pure_filename =
                string.replace(entry.filename, each: path_to_remove, with: "")
              case
                string.contains(pure_filename, "/")
                || string.contains(pure_filename, " ")
              {
                True -> Ok(acc)
                False ->
                  case entry.stat.is_directory {
                    True -> Ok([Directory(pure_filename), ..acc])
                    False -> Ok([File(pure_filename), ..acc])
                  }
              }
            }
            _ -> Error("Failed to read directory")
          }
        })
      })
  }
}

type PathPair {
  PathPair(name: String, path: String)
}

fn get_valid_path(path: String) -> Result(String, String) {
  let paths = get_root_paths()
  io.println(
    list.map(paths, fn(p: PathPair) {
      p.name |> string.append(" - ") |> string.append(p.path)
    })
    |> string.join(", "),
  )
  let valid_path =
    list.find(paths, fn(pair) { string.starts_with(path, pair.name) })
    |> result.try(fn(pair) {
      Ok(string.replace(path, each: pair.name, with: pair.path))
    })
  case valid_path {
    Ok(p) -> Ok(p)
    _ -> Error("Invalid path (not allowed)")
  }
}

fn get_root_paths() -> List(PathPair) {
  list.range(0, 10)
  |> list.fold([], fn(acc, i) {
    let path_env = string.append("PATH_", int.to_string(i))
    let name_env = string.append(path_env, "_NAME")
    env.get_string(path_env)
    |> fn(env_path) {
      case env_path {
        Ok(p) -> [
          PathPair(name: env.get_string_or(name_env, p), path: p),
          ..acc
        ]
        _ -> acc
      }
    }
  })
}
