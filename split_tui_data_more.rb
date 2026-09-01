#!/usr/bin/env ruby
c = File.read("tui_data.go")
idx = c.index("func absDownloadCover")
part1 = c[0...idx]
part2 = "package main\n\nimport (\n\t\"path/filepath\"\n\t\"strconv\"\n\t\"strings\"\n)\n\n" + c[idx..-1]
File.write("tui_data.go", part1)
File.write("tui_data_abs.go", part2)
