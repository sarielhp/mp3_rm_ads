lines = File.readlines('cli_server_cmds.go')
idx = lines.index { |l| l.include?('Name:        "get_info",') } - 1
part1 = lines[0...idx]
part2 = lines[idx..-7]
part1_end = "\t}\n\n\tc.Subcommands = append(c.Subcommands, buildServerSubcommands2(opts, action, countVal, keepVal)...)\n\treturn c\n}\n"
File.write('cli_server_cmds.go', part1.join + part1_end)

part2_full = "package main\n\nimport (\n\t\"github.com/sarielhp/clihelp\"\n)\n\nfunc buildServerSubcommands2(opts *CLIOptions, action *string, countVal *int, keepVal *int) []clihelp.Command {\n\treturn []clihelp.Command{\n" + part2.join + "\t}\n}\n"
File.write('cli_server_cmds2.go', part2_full)
