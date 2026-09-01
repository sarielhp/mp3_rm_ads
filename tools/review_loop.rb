#!/usr/bin/env ruby
# frozen_string_literal: true

bin = [
  File.expand_path('~/bin/agy_review_code_loop'),
  File.expand_path('~/.local/bin/agy_review_code_loop')
].find { |p| File.executable?(p) } || 'agy_review_code_loop'

exec(bin, *ARGV)
