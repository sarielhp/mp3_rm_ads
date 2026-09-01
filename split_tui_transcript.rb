#!/usr/bin/env ruby
lines = File.readlines('tui_transcript_view.go')
File.write('tui_transcript_export.go', "package main\n\nimport (\n\t\"fmt\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n" + lines[245..330].join)
lines.slice!(245..330)
File.write('tui_transcript_view.go', lines.join)
