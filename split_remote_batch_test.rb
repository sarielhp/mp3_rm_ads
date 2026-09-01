#!/usr/bin/env ruby
lines = File.readlines('remote_batch_test.go')
File.write('remote_batch_mock_test.go', "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n" + lines[10..199].join)
lines.slice!(10..199)
File.write('remote_batch_test.go', lines.join)
