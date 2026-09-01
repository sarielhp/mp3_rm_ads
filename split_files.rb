require 'fileutils'

def split_file(file, cut_line, new_file)
  lines = File.readlines(file)
  part1 = lines[0...cut_line]
  part2 = lines[cut_line..-1] # two dots to include the last line!

  pkg_line = lines.find { |l| l.start_with?("package ") }
  imports = []
  in_import = false
  lines.each do |l|
    if l.start_with?("import (")
      in_import = true
      imports << l
    elsif in_import
      imports << l
      if l.strip == ")"
        in_import = false
        break
      end
    elsif l.start_with?("import ") && !l.start_with?("import (")
      imports << l
      break
    end
  end

  part2.unshift("\n")
  part2.unshift(*imports)
  part2.unshift("\n")
  part2.unshift(pkg_line)

  File.write(file, part1.join)
  File.write(new_file, part2.join)
end

split_file("config.go", 387, "config_cli.go")
split_file("tui_keys.go", 405, "tui_keys_nav.go")
split_file("tui_list_view.go", 284, "tui_detail_view.go")
split_file("remote_status.go", 416, "remote_cancel.go")
split_file("kitty.go", 395, "kitty_encode.go")

puts "Done splitting."
