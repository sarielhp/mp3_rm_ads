#!/usr/bin/env ruby
# frozen_string_literal: true

require "open3"
require "fileutils"

root_dir = File.expand_path("..", __dir__)
Dir.chdir(root_dir)

target_binary = File.join(root_dir, "abs")

# Build exclusively into the local repository root
cmd = "go build -o #{target_binary} ."
stdout, stderr, status = Open3.capture3(cmd)

unless status.success?
  warn "\e[31m\u2717 Local build failed!\e[0m"
  warn stdout unless stdout.empty?
  warn stderr unless stderr.empty?
  exit 1
end

# Verify output is strictly within local repository directory
resolved_path = (File.exist?(target_binary) ? File.realpath(target_binary) : target_binary)
unless resolved_path.start_with?(root_dir)
  warn "\e[31m\u2717 Security violation: binary target #{resolved_path} is outside #{root_dir}!\e[0m"
  exit 1
end

File.chmod(0755, target_binary)
puts "Built ./abs successfully."
