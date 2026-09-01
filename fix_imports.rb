require 'open3'
10.times do
  stdout, stderr, status = Open3.capture3("go vet ./...")
  break if status.success?
  stderr.each_line do |line|
    if line =~ /vet: \.\/([^:]+):(\d+):\d+: "([^"]+)" imported and not used/
      file, lineno, pkg = $1, $2.to_i, $3
      lines = File.readlines(file)
      lines[lineno-1] = "// removed #{pkg}\n"
      File.write(file, lines.join)
    end
  end
end
