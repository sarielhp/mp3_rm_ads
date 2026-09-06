#!/usr/bin/env ruby
# frozen_string_literal: true

require 'optparse'

THRESHOLD = 600

options = { threshold: THRESHOLD }
OptionParser.new do |opts|
  opts.banner = "Usage: #{$PROGRAM_NAME} [options] [file.go]"
  opts.on("-t", "--threshold LINES", Integer, "Line threshold for split suggestions (default: #{THRESHOLD})") { |v| options[:threshold] = v }
  opts.on("-h", "--help", "Show this help") do
    puts opts
    exit 0
  end
end.parse!

root_dir = File.expand_path('..', __dir__)
target_files = if ARGV.any?
                 ARGV.map { |f| File.expand_path(f, Dir.pwd) }
               else
                 Dir.glob(File.join(root_dir, '**', '*.go')).reject do |f|
                   f.include?('/.work/') || f.include?('/vendor/') || f.include?('/mail_cli/')
                 end.select { |f| File.readlines(f).size > options[:threshold] }.sort
               end

if target_files.empty?
  puts "\e[32mNo files exceed the #{options[:threshold]}-line threshold for splitting.\e[0m"
  exit 0
end

puts "\e[1m=== File Split Recommendations ===\e[0m\n"

target_files.each do |file|
  lines = File.readlines(file)
  total_lines = lines.size
  rel_path = file.sub("#{root_dir}/", '')

  puts "\e[1;33m► #{rel_path}\e[0m (#{total_lines} lines)"

  # Parse declarations
  decls = []
  i = 0
  while i < lines.size
    line = lines[i]
    start_line = i + 1
    name = nil
    category = :other

    if line =~ /^func\s+\((\w+\s+\*?([A-Za-z]\w*))\)\s+([A-Za-z]\w*)\s*\(/
      recv = Regexp.last_match(2)
      fn = Regexp.last_match(3)
      name = "(#{recv}).#{fn}"
      category = if fn =~ /^handle/
                   :handler
                 elsif fn =~ /^draw|^view|^render/i
                   :view
                 else
                   :method
                 end
    elsif line =~ /^func\s+([A-Za-z]\w*)\s*\(/
      fn = Regexp.last_match(1)
      name = fn
      category = if fn =~ /^parse|build.*App|globalOptions|.*Examples/i
                   :cli
                 elsif fn =~ /^test/i
                   :test_helper
                 else
                   :function
                 end
    elsif line =~ /^type\s+([A-Za-z]\w*)\s+(struct|interface)/
      name = Regexp.last_match(1)
      category = :type
    end

    if name
      brace_count = line.count('{') - line.count('}')
      in_block = brace_count > 0 || line.include?('{')
      j = i + 1
      while j < lines.size && in_block && brace_count > 0
        curr = lines[j]
        brace_count += curr.count('{') - curr.count('}')
        j += 1
      end
      end_line = j
      decl_lines = end_line - start_line + 1
      decls << { name: name, category: category, start: start_line, end: end_line, lines: decl_lines }
      i = j
    else
      i += 1
    end
  end

  # Group suggestions
  groups = decls.group_by { |d| d[:category] }

  if rel_path == 'main.go' && groups[:cli]
    cli_lines = groups[:cli].sum { |d| d[:lines] }
    puts "  \e[36mSuggested Split 1: CLI Flags & Help Definitions\e[0m"
    puts "    Proposed file: \e[1mmain_cli.go\e[0m (~#{cli_lines} lines)"
    puts "    Symbols to move:"
    groups[:cli].each { |d| puts "      - #{d[:name]} (lines #{d[:start]}-#{d[:end]})" }
    puts "    Remaining in main.go: ~#{total_lines - cli_lines} lines"
  elsif rel_path == 'tui.go'
    if groups[:handler]
      handler_lines = groups[:handler].sum { |d| d[:lines] }
      puts "  \e[36mSuggested Split 1: Key & Event Handlers\e[0m"
      puts "    Proposed file: \e[1mtui_handlers.go\e[0m (~#{handler_lines} lines)"
      puts "    Symbols to move:"
      groups[:handler].each { |d| puts "      - #{d[:name]} (lines #{d[:start]}-#{d[:end]})" }
    end
    if groups[:view]
      view_lines = groups[:view].sum { |d| d[:lines] }
      puts "  \e[36mSuggested Split 2: View Rendering\e[0m"
      puts "    Proposed file: \e[1mtui_views.go\e[0m (~#{view_lines} lines)"
      puts "    Symbols to move:"
      groups[:view].each { |d| puts "      - #{d[:name]} (lines #{d[:start]}-#{d[:end]})" }
    end
  else
    # Generic suggestions for test or other files
    puts "  \e[36mDetected Symbol Groups:\e[0m"
    groups.each do |cat, items|
      grp_lines = items.sum { |d| d[:lines] }
      puts "    • Category \e[1m#{cat}\e[0m (#{items.size} symbols, ~#{grp_lines} lines):"
      items.each { |d| puts "      - #{d[:name]} (lines #{d[:start]}-#{d[:end]})" }
    end
  end
  puts ''
end
