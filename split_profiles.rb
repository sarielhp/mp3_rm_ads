lines = File.readlines('profiles.go')
File.write('profiles_whisper.go', "package main\n\nimport (\n\t\"encoding/json\"\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\n" + lines[183..-1].join)
File.write('profiles.go', lines[0..182].join)
