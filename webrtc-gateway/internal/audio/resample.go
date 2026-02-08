package audio

import "encoding/binary"

// Downsample48to16 converts 48kHz mono int16 samples to 16kHz
// by averaging each group of 3 consecutive samples.
func Downsample48to16(in []int16) []int16 {
	out := make([]int16, len(in)/3)
	for i := range out {
		sum := int32(in[i*3]) + int32(in[i*3+1]) + int32(in[i*3+2])
		out[i] = int16(sum / 3)
	}
	return out
}

// Upsample16to48 converts 16kHz mono int16 samples to 48kHz
// by repeating each sample 3 times.
func Upsample16to48(in []int16) []int16 {
	out := make([]int16, len(in)*3)
	for i, s := range in {
		out[i*3] = s
		out[i*3+1] = s
		out[i*3+2] = s
	}
	return out
}

// Int16ToBytes converts int16 samples to s16le byte slice.
func Int16ToBytes(samples []int16) []byte {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return buf
}

// BytesToInt16 converts s16le byte slice to int16 samples.
func BytesToInt16(data []byte) []int16 {
	samples := make([]int16, len(data)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return samples
}
