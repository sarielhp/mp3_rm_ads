#!/usr/bin/env ruby
c = File.read("tui_data_queue.go")
c.sub!(/^\s*"time"\n/m, "")
File.write("tui_data_queue.go", c)
