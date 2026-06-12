package audio

import (
	"cmp"
	"math"
)

func clamp[T cmp.Ordered](value T, low T, high T) T {
	return max(min(value, high), low)
}

// Voice-activity dial: the config exposes a 1-100 knob that maps linearly onto
// the RMS gate used by IsAboveThreshold. 1 = most sensitive, 100 = least.
const (
	VoiceKnobMin = 1.0
	VoiceKnobMax = 100.0
	voiceRMSMin  = 0.0001
	voiceRMSMax  = 0.01
)

// VoiceKnobToRMS maps a [1,100] sensitivity knob to its RMS threshold.
func VoiceKnobToRMS(knob float64) float64 {
	knob = clamp(knob, VoiceKnobMin, VoiceKnobMax)
	return voiceRMSMin + (knob-VoiceKnobMin)/(VoiceKnobMax-VoiceKnobMin)*(voiceRMSMax-voiceRMSMin)
}

// IsAboveThreshold computes the RMS of 16-bit PCM data and compares it against the threshold
func IsAboveThreshold(pcm []int16, threshold float64) bool {
	if len(pcm) == 0 {
		return false
	}

	var sum float64
	for _, sample := range pcm {
		// Normalize to [-1.0, 1.0]
		normalized := float64(sample) / 32768.0
		sum += normalized * normalized
	}

	numSamples := float64(len(pcm))
	rms := math.Sqrt(sum / numSamples)
	return rms >= threshold
}
