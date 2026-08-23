#!/usr/bin/env ruby
# frozen_string_literal: true

require 'optparse'

options = { filter: nil, file_filter: nil }
OptionParser.new do |opts|
  opts.banner = "Usage: #{$PROGRAM_NAME} [options] [pattern]"
  opts.on("-s", "--symbol PATTERN", "Filter symbols by name/regex") { |v| options[:filter] = v }
  opts.on("-f", "--file PATTERN", "Filter by file name/regex") { |v| options[:file_filter] = v }
  opts.on("-h", "--help", "Show this help") do
    puts opts
    exit 0
  end
end.parse!

if ARGV.any? && !options[:filter]
  options[:filter] = ARGV[0]
end

root_dir = File.expand_path('..', __dir__)
go_files = Dir.glob(File.join(root_dir, '**', '*.go')).reject do |f|
  f.include?('/.work/') || f.include?('/vendor/') || f.include?('/mail_cli/') || f.end_with?('_test.go')
end.sort

if options[:file_filter]
  rx = Regexp.new(options[:file_filter], Regexp::IGNORECASE)
  go_files = go_files.select { |f| f =~ rx }
end

symbol_rx = options[:filter] ? Regexp.new(options[:filter], Regexp::IGNORECASE) : nil

puts "\e[1m=== Go Codebase Symbol Outline ===\e[0m\n\n"

go_files.each do |file|
  rel_path = file.sub("#{root_dir}/", '')
  symbols = []

  File.readlines(file).each_with_index do |line, idx|
    lineno = idx + 1
    case line
    when /^type\s+([A-Za-z]\w*)\s+(struct|interface)/
      sym = { name: Regexp.last_match(1), kind: Regexp.last_match(2), line: lineno }
      symbols << sym if symbol_rx.nil? || sym[:name] =~ symbol_rx
    when /^type\s+([A-Za-z]\w*)\s+(\w+)/
      sym = { name: Regexp.last_match(1), kind: "type (#{Regexp.last_match(2)})", line: lineno }
      symbols << sym if symbol_rx.nil? || sym[:name] =~ symbol_rx
    when /^func\s+\((\w+\s+\*?([A-Za-z]\w*))\)\s+([A-Za-z]\w*)\s*\(/
      recv = Regexp.last_match(2)
      fn = Regexp.last_match(3)
      sym = { name: "(#{recv}).#{fn}", kind: 'method', line: lineno }
      symbols << sym if symbol_rx.nil? || sym[:name] =~ symbol_rx || fn =~ symbol_rx
    when /^func\s+([A-Za-z]\w*)\s*\(/
      fn = Regexp.last_match(1)
      sym = { name: fn, kind: 'func', line: lineno }
      symbols << sym if symbol_rx.nil? || sym[:name] =~ symbol_rx
    end
  end

  next if symbols.empty?

  puts "\e[1;34m#{rel_path}\e[0m"
  symbols.each do |sym|
    kind_colored = case sym[:kind]
                   when 'struct' then "\e[32mstruct\e[0m"
                   when 'interface' then "\e[35minterface\e[0m"
                   when 'method' then "\e[36mmethod\e[0m"
                   when 'func' then "\e[33mfunc\e[0m"
                   else "\e[37m#{sym[:kind]}\e[0m"
                   end
    puts format("  line %-4d %-18s %s", sym[:line], kind_colored, sym[:name])
  end
  puts ''
end
