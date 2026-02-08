package audio

import "math"

const (
	ToneFrequency  = 440.0
	ToneSampleRate = 16000
	ToneAmplitude  = 16000
)

// GenerateSineWave produces a sine wave at the given frequency and duration
// as 16kHz mono int16 PCM samples.
func GenerateSineWave(durationSec, frequency float64) []int16 {
	numSamples := int(durationSec * ToneSampleRate)
	samples := make([]int16, numSamples)
	for i := range samples {
		t := float64(i) / ToneSampleRate
		samples[i] = int16(ToneAmplitude * math.Sin(2*math.Pi*frequency*t))
	}
	return samples
}
