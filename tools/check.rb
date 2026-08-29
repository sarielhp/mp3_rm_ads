#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'

root_dir = File.expand_path('..', __dir__)
Dir.chdir(root_dir)

def run_cmd(name, cmd)
  stdout, stderr, status = Open3.capture3(cmd)
  unless status.success?
    puts "\e[31m✗ #{name} failed!\e[0m"
    puts "\e[1mStdout:\e[0m"
    puts stdout unless stdout.empty?
    puts "\e[1mStderr:\e[0m"
    puts stderr unless stderr.empty?
    exit 1
  end
  [stdout, stderr]
end

# 1. Format
run_cmd("Formatting", "gofmt -s -w .")
run_cmd("Config template generation", "ruby tools/generate_config_template.rb")

# 2. Tidy
run_cmd("Go mod tidy", "go mod tidy")

# 3. Vet
run_cmd("Go vet", "go vet ./...")

# 4. Staticcheck
run_cmd("Staticcheck", "staticcheck -checks '-SA2001' ./...")

# 5. Audit lines
stdout, _ = run_cmd("Line audit", "ruby tools/audit_lines.rb --quiet")
puts stdout.strip unless stdout.strip.empty?

# 6. Go test
run_cmd("Go test", "go test -timeout 30s ./...")

# 7. Build
run_cmd("Go build", "go build -o abs .")

puts "Success: Quality Gate Passed"
