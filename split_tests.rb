#!/usr/bin/env ruby

def split_test(filename)
  content = File.read(filename)
  # Find the middle func Test
  lines = content.lines
  funcs = lines.each_with_index.select { |l, i| l.start_with?("func Test") }
  mid_func = funcs[funcs.size / 2]
  idx = content.index(mid_func[0])
  
  part1 = content[0...idx]
  # Grab imports from part1 for part2
  imports = content.match(/import \(\n.*?\n\)/m)
  imports_str = imports ? imports[0] : "import \"testing\""
  
  part2 = "package main\n\n#{imports_str}\n\n" + content[idx..-1]
  
  File.write(filename, part1)
  File.write(filename.sub(".go", "_extra.go"), part2)
end

split_test("config_test.go")
split_test("main_test.go")
split_test("main_cli_test.go")
