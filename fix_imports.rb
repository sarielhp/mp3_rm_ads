#!/usr/bin/env ruby

part2 = File.read("cli_server_cmds_extra.go")
part2 = part2.sub(/import \(/, "import (\n\t\"fmt\"")
File.write("cli_server_cmds_extra.go", part2)
