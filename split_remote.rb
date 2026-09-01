lines = File.readlines('cli_remote_cmds.go')
File.write('cli_remote_exec.go', "package main\n\nimport (\n\t\"fmt\"\n)\n\n" + lines[255..-1].join)
File.write('cli_remote_cmds.go', lines[0..254].join)
