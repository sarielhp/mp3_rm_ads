#!/usr/bin/env ruby

content = File.read("tui_data.go")
idx = content.index("func loadAllQueues")
part1 = content[0...idx]
part2 = "package main\n\nimport (\n\t\"fmt\"\n\t\"time\"\n)\n\n" + content[idx..-1]

File.write("tui_data.go", part1)
File.write("tui_data_queue.go", part2)
