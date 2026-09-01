def split_test_file(filename, new_filename)
  lines = File.readlines(filename)
  total = lines.size
  split_point = total / 2

  # Find the next 'func Test' after the halfway point
  func_start = nil
  lines.each_with_index do |line, idx|
    if idx > split_point && line.start_with?("func Test")
      func_start = idx
      break
    end
  end

  return unless func_start

  header = lines[0...10] # just grab package and imports roughly
  imports_end = lines.index { |l| l.strip == ")" }
  if imports_end
    header = lines[0..imports_end]
  else
    # find first func
    first_func = lines.index { |l| l.start_with?("func ") }
    header = lines[0...first_func]
  end

  file1 = lines[0...func_start].join
  file2 = header.join + "\n" + lines[func_start..-1].join

  File.write(filename, file1)
  File.write(new_filename, file2)
end

split_test_file("tui_screens_test.go", "tui_screens_extra_test.go")
split_test_file("misc_test.go", "misc_extra_test.go")
