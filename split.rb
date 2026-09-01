lines = File.readlines('pm_frequency_test.go')
File.write('pm_frequency_analyze_test.go', "package main\n\nimport (\n\t\"testing\"\n\t\"time\"\n)\n\n" + lines[10..233].join)
File.write('pm_frequency_test.go', lines[0..9].join + lines[234..-1].join)
