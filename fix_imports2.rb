#!/usr/bin/env ruby

c = File.read("cli_server_cmds.go")
c.gsub!(/^\s*"fmt"\n/, "")
c.gsub!(/^\s*"strconv"\n/, "")
c.gsub!(/^\s*"strings"\n/, "")
File.write("cli_server_cmds.go", c)
