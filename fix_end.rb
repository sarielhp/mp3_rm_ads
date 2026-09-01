#!/usr/bin/env ruby

content = File.read("cli_server_cmds.go")
content = content.sub(/		\},\n	\}\n\}\n\z/, "\t}\n}\n")
File.write("cli_server_cmds.go", content)
