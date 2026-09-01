#!/usr/bin/env ruby

content = File.read("cli_server_cmds.go")
content.gsub!(/func buildServerCommand\(opts \*CLIOptions, action \*string, countVal, keepVal \*int\) clihelp\.Command \{/, "func buildServerCommand(opts *CLIOptions, action *string, countVal, keepVal *int) clihelp.Command {\n\treturn clihelp.Command{\n\t\tName:        \"server\",\n\t\tDescription: \"Manage and interact with the Audiobookshelf server\",\n\t\tSubcommands: buildServerSubcommands(opts, action, countVal, keepVal),\n\t}\n}\n\nfunc buildServerSubcommands(opts *CLIOptions, action *string, countVal, keepVal *int) []clihelp.Command {\n\treturn []clihelp.Command{")

# Now I need to fix the return statement at the end.
# It used to end with:
# 	}
# }
content.sub!(/\t\}\n\}\n\z/, "\t}\n}\n")
File.write("cli_server_cmds.go", content)
