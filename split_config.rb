content = File.read("config.go")
methods_to_extract = ["func userTmpDir", "func configDir", "func configPath", "func legacyConfigPath", "func opencodeConfigPath", "func localIP", "func replaceIP"]
# We'll just extract these functions and put them in config_path.go
extracted = ""
remaining = []

in_extract = false
brace_count = 0

content.lines.each do |line|
  if !in_extract
    if methods_to_extract.any? { |m| line.start_with?(m) }
      in_extract = true
      brace_count = line.count("{") - line.count("}")
      extracted += line
    else
      remaining << line
    end
  else
    extracted += line
    brace_count += line.count("{") - line.count("}")
    if brace_count == 0
      in_extract = false
      extracted += "\n"
    end
  end
end

File.write("config.go", remaining.join)
File.write("config_path.go", "package main\n\nimport (\n\t\"fmt\"\n\t\"net\"\n\t\"os\"\n\t\"os/user\"\n\t\"path/filepath\"\n\t\"strings\"\n)\n\n" + extracted)
