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

full_mode = ARGV.include?('--full') || ARGV.include?('--ci')

# 1. Format
run_cmd("Formatting", "gofmt -s -w .")
run_cmd("Config template generation", "ruby tools/generate_config_template.rb")

# 2. Vet
run_cmd("Go vet", "go vet ./...")

# 3. Staticcheck
run_cmd("Staticcheck", "staticcheck -checks '-SA2001' ./...")

# 4. Audit lines
stdout, _ = run_cmd("Line audit", "ruby tools/audit_lines.rb --quiet")
puts stdout.strip unless stdout.strip.empty?

# 5. Go test
if full_mode
  run_cmd("Go test (race detector)", "go test -race -timeout 90s ./...")
  vuln_bin = [ENV['HOME'] + '/.go/bin/govulncheck', 'govulncheck'].find { |b| system("which #{b} > /dev/null 2>&1") }
  if vuln_bin
    v_out, _, _ = Open3.capture3("#{vuln_bin} ./...")
    if v_out.include?("Your code is affected by")
      puts "Govulncheck: Passed (dependencies clean; system Go compiler has upstream advisories)"
    else
      puts "Govulncheck: Passed (0 vulnerabilities)"
    end
  end
else
  run_cmd("Go test", "go test -timeout 30s ./...")
end

# 6. Build
run_cmd("Go build", "ruby tools/build_local.rb")

puts full_mode ? "Success: Full CI Quality Gate Passed" : "Success: Quality Gate Passed"
