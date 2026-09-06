#!/usr/bin/env ruby
# frozen_string_literal: true

require 'optparse'

if ARGV.empty?
  puts "Usage: #{$PROGRAM_NAME} <symbol_name_or_regex>"
  puts "Example: #{$PROGRAM_NAME} loadConfig"
  puts "Example: #{$PROGRAM_NAME} Config"
  exit 1
end

query = ARGV[0]
query_rx = Regexp.new("\\b#{Regexp.escape(query)}\\b", Regexp::IGNORECASE)

root_dir = File.expand_path('..', __dir__)
go_files = Dir.glob(File.join(root_dir, '**', '*.go')).reject do |f|
  f.include?('/.work/') || f.include?('/vendor/') || f.include?('/mail_cli/')
end.sort

matches_found = 0

go_files.each do |file|
  lines = File.readlines(file)
  rel_path = file.sub("#{root_dir}/", '')

  i = 0
  while i < lines.size
    line = lines[i]
    is_decl = false
    sym_name = ''

    if line =~ /^type\s+([A-Za-z]\w*)\s+(struct|interface)/
      sym_name = Regexp.last_match(1)
      is_decl = true if sym_name =~ query_rx
    elsif line =~ /^type\s+([A-Za-z]\w*)\s+\w+/
      sym_name = Regexp.last_match(1)
      is_decl = true if sym_name =~ query_rx
    elsif line =~ /^func\s+\([^)]+\)\s+([A-Za-z]\w*)\s*\(/
      sym_name = Regexp.last_match(1)
      is_decl = true if sym_name =~ query_rx
    elsif line =~ /^func\s+([A-Za-z]\w*)\s*\(/
      sym_name = Regexp.last_match(1)
      is_decl = true if sym_name =~ query_rx
    end

    if is_decl
      matches_found += 1
      start_line = i + 1
      block_lines = [line]

      brace_count = line.count('{') - line.count('}')
      in_block = brace_count > 0 || line.include?('{')

      j = i + 1
      while j < lines.size && (in_block && brace_count > 0)
        curr = lines[j]
        block_lines << curr
        brace_count += curr.count('{') - curr.count('}')
        j += 1
      end

      # For single line type alias
      j += 1 if !in_block && j == i + 1

      end_line = start_line + block_lines.size - 1

      puts "\e[1;34m=== #{rel_path}:#{start_line}-#{end_line} (#{sym_name}) ===\e[0m"
      block_lines.each_with_index do |bl, offset|
        puts format("\e[90m%4d\e[0m | %s", start_line + offset, bl)
      end
      puts ''

      i = j
    else
      i += 1
    end
  end
end

if matches_found.zero?
  puts "No symbol definitions matching '#{query}' found."
  exit 1
end
