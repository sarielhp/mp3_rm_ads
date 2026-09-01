#!/usr/bin/env ruby
lines = File.readlines('player.go')
File.write('player_ui.go', "package main\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\n" + lines[407..479].join)
lines.slice!(407..479)
File.write('player.go', lines.join)
