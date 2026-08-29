#!/usr/bin/env ruby
# frozen_string_literal: true

require 'open3'

root_dir = File.expand_path('..', __dir__)
Dir.chdir(root_dir)

if ARGV.empty?
  puts "Usage: #{$PROGRAM_NAME} <commit-message>"
  exit 1
end

msg = ARGV.join(' ')

# Run quality gate silently
_, _, status = Open3.capture3("ruby tools/check.rb")
unless status.success?
  puts "✗ Quality Gate failed! Run 'make check' to see details."
  exit 1
end

# Commit
Open3.capture3("git add -A")
Open3.capture3("git commit -m '#{msg}'")

puts "Success #{msg}"
