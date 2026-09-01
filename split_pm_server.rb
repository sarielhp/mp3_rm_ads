lines = File.readlines('pm_server_exec.go')
File.write('pm_server_exec2.go', "package main\n\nfunc handleServerCommandPart2(config Config, cli CLIOptions, targetFile string, targetDir string) {\n\tswitch cli.ServerSubcmd {\n" + lines[157..-2].join + "\n}\n")
File.write('pm_server_exec.go', lines[0..156].join + "\n\tdefault:\n\t\thandleServerCommandPart2(config, cli, targetFile, targetDir)\n\t}\n}\n")
