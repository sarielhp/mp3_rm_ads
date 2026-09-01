#!/usr/bin/env ruby
lines = File.readlines('format.go')
File.write('format_json.go', "package main\n\nimport (\n\t\"encoding/json\"\n\t\"os\"\n\t\"path/filepath\"\n)\n\n" + lines[390..431].join)
File.write('format_intervals.go', "package main\n\nimport (\n\t\"sort\"\n)\n\n" + lines[189..276].join + "\n" + lines[432..493].join)
lines.slice!(432..493)
lines.slice!(390..431)
lines.slice!(189..276)
File.write('format.go', lines.join)
