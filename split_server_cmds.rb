#!/usr/bin/env ruby

content = File.read("cli_server_cmds.go")

idx = content.index("func buildServerSubcommands")
part1 = content[0...idx]
part2 = "package main\n\nimport (\n\t\"strconv\"\n\t\"strings\"\n\t\"github.com/sarielhp/clihelp\"\n)\n\n" + content[idx..-1]

File.write("cli_server_cmds.go", part1)
File.write("cli_server_cmds_extra.go", part2)
