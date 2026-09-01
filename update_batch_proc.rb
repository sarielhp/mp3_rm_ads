content = File.read("batch_proc.go")
old_loop = <<~GO
		errFlag, processedFlag := processSingleAudioFile(idx, len(expandedArgs), inputFile, cli, config, action, batchStartTime, selectedProfile)
		if errFlag {
			hasError = true
		}
		if processedFlag {
			processedCount++
		}
GO
new_loop = <<~GO
		errFlag, processedFlag, stopFlag := processSingleAudioFile(idx, len(expandedArgs), processedCount, inputFile, cli, config, action, batchStartTime, selectedProfile)
		if errFlag {
			hasError = true
		}
		if stopFlag {
			break
		}
		if processedFlag {
			processedCount++
		}
GO
content = content.sub(old_loop, new_loop)
File.write("batch_proc.go", content)
