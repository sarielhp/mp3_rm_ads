#!/usr/bin/env ruby

content = File.read("tui_text_utils.go")
content.sub!("import (\n\t\"strings\"\n)", "import (\n\t\"strings\"\n\t\"github.com/charmbracelet/lipgloss\"\n)")
File.write("tui_text_utils.go", content)
