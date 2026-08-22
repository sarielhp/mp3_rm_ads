#!/usr/bin/env ruby
# frozen_string_literal: true

require 'optparse'

MAX_LINES = 600
SOFT_LIMIT = 300

options = { max: MAX_LINES, soft: SOFT_LIMIT, quiet: false, strict: false }
OptionParser.new do |opts|
  opts.banner = "Usage: #{$PROGRAM_NAME} [options]"
  opts.on("-m", "--max LINES", Integer, "Hard limit for file lines (default: #{MAX_LINES})") { |v| options[:max] = v }
  opts.on("-s", "--soft LINES", Integer, "Soft target limit for file lines (default: #{SOFT_LIMIT})") { |v| options[:soft] = v }
  opts.on("--strict", "Exit with error code if any file exceeds hard limit") { options[:strict] = true }
  opts.on("-q", "--quiet", "Show only warnings and violations") { options[:quiet] = true }
  opts.on("-h", "--help", "Show this help") do
    puts opts
    exit 0
  end
end.parse!

root_dir = File.expand_path('..', __dir__)
go_files = Dir.glob(File.join(root_dir, '**', '*.go')).reject do |f|
  f.include?('/.work/') || f.include?('/vendor/') || f.include?('/mail_cli/')
end.sort

violations = []
warnings = []
passed = []

go_files.each do |file|
  lines = File.readlines(file).size
  rel_path = file.sub("#{root_dir}/", '')
  entry = { path: rel_path, lines: lines }

  if lines > options[:max]
    violations << entry
  elsif lines > options[:soft]
    warnings << entry
  else
    passed << entry
  end
end

puts "\e[1m=== Go File Sizing Audit ===\e[0m"
puts "Target: 150-#{options[:soft]} lines | Hard limit: #{options[:max]} lines\n\n"

unless options[:quiet]
  passed.each do |e|
    puts format("  \e[32m[PASS]\e[0m %4d lines  %s", e[:lines], e[:path])
  end
end

warnings.each do |e|
  puts format("  \e[33m[WARN]\e[0m %4d lines  %s (approaching limit)", e[:lines], e[:path])
end

violations.each do |e|
  puts format("  \e[31m[WARN]\e[0m %4d lines  %s (exceeds #{options[:max]} lines — recommend splitting)", e[:lines], e[:path])
end

puts "\n\e[1mSummary:\e[0m #{passed.size} pass, #{warnings.size} warnings, #{violations.size} over #{options[:max]} lines (#{go_files.size} total files)"

if options[:strict] && violations.any?
  warn "\n\e[31mError: #{violations.size} file(s) exceed #{options[:max]}-line limit in strict mode.\e[0m"
  exit 1
else
  exit 0
end
