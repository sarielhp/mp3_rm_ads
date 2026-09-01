lines = File.readlines('cli_server_cmds.go')
idx = lines.index { |l| l.include?('Name:        "get_info",') } - 2
part1 = lines[0...idx]
part2 = lines[idx..-7]
File.write('cli_server_cmds2.go', "package main\n\nimport (\n\t\"github.com/sariel/abs/pkg/clihelp\"\n)\n\nfunc buildServerSubcommands2(opts *CLI, action *string, countVal *int) []clihelp.Command {\n\treturn []clihelp.Command{\n" + part2.join + "\t}\n}\n")

new_part1 = part1.join + "\t}\n\n\tc.Subcommands = append(c.Subcommands, buildServerSubcommands2(opts, action, countVal)...)\n\treturn c\n}\n"
File.write('cli_server_cmds.go', new_part1)
