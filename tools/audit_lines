#!/usr/bin/env ruby
# frozen_string_literal: true

require 'optparse'

# File length is a comfort metric; function length is a correctness metric.
#
# Splitting a file is a physical edit — a byte range is moved and nothing checks
# that control flow survived. Two such splits in this repo's history silently
# dropped a `continue` (c9b68f4) and turned `return len(...)` into `return 0`
# (1c49c7b); both compiled and both passed the gate. Splitting a *function* is a
# semantic edit: the extracted piece needs a name, parameters and returns, and
# the compiler checks every call site. A dropped return becomes a build error.
#
# So the function limit is the one to obey when the two conflict: decompose the
# long function in place rather than cutting the file underneath it.

MAX_LINES = 1100
SOFT_LIMIT = 800
MAX_FUNC_LINES = 80

options = {
  max: MAX_LINES,
  soft: SOFT_LIMIT,
  max_func: MAX_FUNC_LINES,
  quiet: false,
  strict: false,
  include_tests: false
}
OptionParser.new do |opts|
  opts.banner = "Usage: #{$PROGRAM_NAME} [options]"
  opts.on("-m", "--max LINES", Integer, "Hard limit for file lines (default: #{MAX_LINES})") { |v| options[:max] = v }
  opts.on("-s", "--soft LINES", Integer, "Warning threshold for file lines (default: #{SOFT_LIMIT})") { |v| options[:soft] = v }
  opts.on("-f", "--max-func LINES", Integer, "Hard limit for function lines (default: #{MAX_FUNC_LINES})") { |v| options[:max_func] = v }
  opts.on("--include-tests", "Apply the function limit to _test.go files too") { options[:include_tests] = true }
  opts.on("--strict", "Exit with error code if any file or function exceeds its hard limit") { options[:strict] = true }
  opts.on("-q", "--quiet", "Show only warnings and violations") { options[:quiet] = true }
  opts.on("-h", "--help", "Show this help") do
    puts opts
    exit 0
  end
end.parse!

# Measure a gofmt'd Go function: it opens with `func` in column 0 and closes with
# `}` in column 0. Single-line bodies (`func f() {}`) never open a block.
def scan_functions(lines)
  funcs = []
  lines.each_with_index do |line, idx|
    next unless line.start_with?('func ') || line.start_with?('func(')
    next unless line.rstrip.end_with?('{')

    close = nil
    ((idx + 1)...lines.size).each do |j|
      if lines[j].rstrip == '}'
        close = j
        break
      end
    end
    next if close.nil?

    funcs << { name: func_name(line), line: idx + 1, lines: close - idx + 1 }
  end
  funcs
end

def func_name(decl)
  sig = decl.sub(/\A func \s* /x, '')
  sig = sig.sub(/\A\([^)]*\)\s*/, '') # strip a method receiver
  name = sig[/\A[A-Za-z0-9_]+/]
  name || '(anonymous)'
end

root_dir = File.expand_path('..', __dir__)
go_files = Dir.glob(File.join(root_dir, '**', '*.go')).reject do |f|
  f.include?('/.work/') || f.include?('/vendor/') || f.include?('/mail_cli/')
end.sort

violations = []
warnings = []
passed = []
long_funcs = []

go_files.each do |file|
  lines = File.readlines(file)
  rel_path = file.sub("#{root_dir}/", '')
  entry = { path: rel_path, lines: lines.size }

  if lines.size > options[:max]
    violations << entry
  elsif lines.size > options[:soft]
    warnings << entry
  else
    passed << entry
  end

  next if rel_path.end_with?('_test.go') && !options[:include_tests]

  scan_functions(lines).each do |fn|
    long_funcs << fn.merge(path: rel_path) if fn[:lines] > options[:max_func]
  end
end

puts "\e[1m=== Go Sizing Audit ===\e[0m"
puts "Functions: hard limit #{options[:max_func]} lines"
puts "Files: warn over #{options[:soft]} lines | hard limit #{options[:max]} lines\n\n"

QUIET_FUNC_ROWS = 10

puts "\e[1mFunctions\e[0m"
if long_funcs.empty?
  puts "  \e[32m[PASS]\e[0m no function exceeds #{options[:max_func]} lines"
else
  ranked = long_funcs.sort_by { |f| -f[:lines] }
  shown = options[:quiet] ? ranked.first(QUIET_FUNC_ROWS) : ranked
  shown.each do |f|
    puts format("  \e[31m[FAIL]\e[0m %4d lines  %s:%d %s()", f[:lines], f[:path], f[:line], f[:name])
  end
  if shown.size < ranked.size
    puts format("  \e[31m[FAIL]\e[0m ... and %d more (run `make audit` for the full list)", ranked.size - shown.size)
  end
  puts "\n  Decompose these in place — extract named helpers into the same file."
  puts "  Do not split the file through a function body; that is the edit that loses control flow."
end

puts "\n\e[1mFiles\e[0m"
unless options[:quiet]
  passed.each do |e|
    puts format("  \e[32m[PASS]\e[0m %4d lines  %s", e[:lines], e[:path])
  end
end

warnings.each do |e|
  puts format("  \e[33m[WARN]\e[0m %4d lines  %s (over #{options[:soft]} lines)", e[:lines], e[:path])
end

violations.each do |e|
  puts format("  \e[31m[FAIL]\e[0m %4d lines  %s (exceeds #{options[:max]} lines)", e[:lines], e[:path])
end

if warnings.empty? && violations.empty? && options[:quiet]
  puts "  \e[32m[PASS]\e[0m no file exceeds #{options[:soft]} lines"
end

puts "\n\e[1mSummary:\e[0m #{go_files.size} files — #{passed.size} pass, " \
     "#{warnings.size} over #{options[:soft]}, #{violations.size} over #{options[:max]}; " \
     "#{long_funcs.size} function(s) over #{options[:max_func]} lines"

if options[:strict] && (violations.any? || long_funcs.any?)
  warn "\n\e[31mError: #{violations.size} file(s) and #{long_funcs.size} function(s) exceed their hard limit in strict mode.\e[0m"
  exit 1
end
exit 0
