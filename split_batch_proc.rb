content = File.read("batch_proc.go")

loop_start = content.index("for idx, inputFile := range expandedArgs {")
loop_end = content.rindex("if (processedCount > 1 || totalFiles > 1) && !cli.Quiet {") - 2

loop_end = content[0..loop_end].rindex("}")

before_loop = content[0...loop_start]
loop_body = content[loop_start...loop_end+1]
after_loop = content[loop_end+1..-1]

inner_body = loop_body.sub(/for idx, inputFile := range expandedArgs \{\n/, "")
inner_body = inner_body.sub(/\}\z/, "")

# Replace `continue` with `return` appropriately. The script previously replaced continue with return hasError, processed, but wait, if we return early, processed might be false but wait, inside the loop originally, did they do `processedCount++`? Let's check original.
