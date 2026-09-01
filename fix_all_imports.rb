#!/usr/bin/env ruby

10.times do
  output = `make lint 2>&1`
  if $?.success?
    break
  end
  output.lines.each do |line|
    if line =~ /vet: \.\/(.+?):(\d+):\d+: "(.*?)" imported and not used/
      file = $1
      pkg = $3
      c = File.read(file)
      c.sub!(/^\s*"#{Regexp.escape(pkg)}"\n/m, "")
      File.write(file, c)
    elsif line =~ /vet: \.\/(.+?):(\d+):\d+: undefined: (.*?)$/
      file = $1
      pkg = $3
      c = File.read(file)
      # Guess package from name
      guess = pkg
      guess = "fmt" if pkg == "fmt"
      guess = "strings" if pkg == "strings"
      guess = "os" if pkg == "os"
      guess = "path/filepath" if pkg == "filepath"
      guess = "net/http" if pkg == "http" || pkg == "httpTest"
      guess = "time" if pkg == "time"
      guess = "io" if pkg == "io"
      
      c.sub!(/import \(\n/, "import (\n\t\"#{guess}\"\n")
      File.write(file, c)
    end
  end
  system("make format")
end
