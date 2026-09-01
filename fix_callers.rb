files = ['batch_proc_file.go', 'main.go', 'pipeline.go', 'remote_worker.go', 'tui_transcript_export.go', 'status_report.go', 'remote_clear.go', 'remote_batch.go', 'ls_cmd.go', 'podcast_id.go', 'pm_download.go', 'main_cli_test.go', 'batch_proc.go', 'misc_test.go']

files.each do |file|
  next unless File.exist?(file)
  content = File.read(file)
  
  content.gsub!(/convertJSONToSRT\(([^)]+)\)\n/) { |m| "_, _ = convertJSONToSRT(#{$1})\n" }
  content.gsub!(/convertJSONToTXT\(([^)]+)\)\n/) { |m| "_, _ = convertJSONToTXT(#{$1})\n" }
  
  content.gsub!(/copyFile\(([^)]+)\)\n/) { |m| m.include?("err :=") ? m : "_ = copyFile(#{$1})\n" }
  
  content.gsub!(/(\w+)\s*:=\s*findMP3Files\(([^)]+)\)/, '\1, _ := findMP3Files(\2)')
  content.gsub!(/for _, (\w+)\s*:=\s*range findMP3Files\(([^)]+)\)/, "files_tmp, _ := findMP3Files(\\2)\n\tfor _, \\1 := range files_tmp")

  content.gsub!(/(\w+)\s*:=\s*getAudioDuration\(([^)]+)\)/, '\1, _ := getAudioDuration(\2)')
  content.gsub!(/GetDuration:\s*getAudioDuration/, 'GetDuration: func(path string) float64 { d, _ := getAudioDuration(path); return d }')
  
  content.gsub!(/if cutAudioFFmpegWithHost\(([^)]+)\)/, 'if err := cutAudioFFmpegWithHost(\1); err == nil')
  content.gsub!(/if cutAudioFFmpeg\(([^)]+)\)/, 'if err := cutAudioFFmpeg(\1); err == nil')

  File.write(file, content)
end
