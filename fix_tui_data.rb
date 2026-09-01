#!/usr/bin/env ruby
c = File.read("tui_data.go")
c.sub!(/^\s*"encoding\/json"\n/m, "")
File.write("tui_data.go", c)
