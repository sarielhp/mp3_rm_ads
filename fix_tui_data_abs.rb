#!/usr/bin/env ruby
c = File.read("tui_data_abs.go")
c.sub!(/import \(\n.*?\n\)/m, "import (\n\t\"path/filepath\"\n\t\"strconv\"\n\t\"strings\"\n\t\"github.com/sariel/abs/pkg/backend\"\n)")
File.write("tui_data_abs.go", c)
