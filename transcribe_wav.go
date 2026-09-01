package main

func buildWavHeader(dataSize int) []byte {
	channels := uint16(1)
	bitsPerSample := uint16(16)
	sampleRate := uint32(wavSampleRate)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	blockAlign := channels * bitsPerSample / 8
	riffSize := uint32(36 + dataSize)

	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	putUint32(header[4:8], riffSize)
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	putUint32(header[16:20], 16)
	putUint16(header[20:22], 1)
	putUint16(header[22:24], channels)
	putUint32(header[24:28], sampleRate)
	putUint32(header[28:32], byteRate)
	putUint16(header[32:34], blockAlign)
	putUint16(header[34:36], bitsPerSample)
	copy(header[36:40], []byte("data"))
	putUint32(header[40:44], uint32(dataSize))

	return header
}

func putUint16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putUint32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
