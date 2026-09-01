#!/usr/bin/env ruby

content = File.read("tui_episode_view.go")
idx = content.index("func wrapDescription")
part1 = content[0...idx]
part2 = "package main\n\nimport (\n\t\"strings\"\n)\n\n" + content[idx..-1]

File.write("tui_episode_view.go", part1)
File.write("tui_text_utils.go", part2)
