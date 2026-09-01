require 'fileutils'

def extract(file, start_line, end_line, out_file, imports)
  lines = File.readlines(file)
  extracted = lines[(start_line-1)..(end_line-1)]
  File.write(out_file, "package main\n\n#{imports}\n" + extracted.join)
  
  lines.slice!((start_line-1)..(end_line-1))
  File.write(file, lines.join)
end

extract('transcribe.go', 18, 54, 'transcribe_wav.go', "")
extract('player.go', 408, 479, 'player_ui.go', "import (\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
extract('tui_transcript_view.go', 264, 330, 'tui_transcript_export.go', "import (\n\t\"fmt\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n")
extract('remote_batch_test.go', 18, 199, 'remote_batch_mock_test.go', "import (\n\t\"fmt\"\n\t\"os\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n")

# for format.go, multiple extractions
lines = File.readlines('format.go')
part1 = lines[432..493]
part2 = lines[189..275]
File.write('format_intervals.go', "package main\n\nimport (\n\t\"sort\"\n)\n\n" + part2.join + "\n" + part1.join)
lines.slice!(432..493)
lines.slice!(189..275)
File.write('format.go', lines.join)
