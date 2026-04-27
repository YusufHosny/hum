package audio

import "time"

const (
	DefaultSampleRate    = 48000 // 48kHz is standard for Opus VoIP
	DefaultChannels      = 1     // Mono audio is usually sufficient for voice chat
	DefaultFrameDuration = 20 * time.Millisecond // 20ms frame is the Opus standard
	DefaultBitrate       = 24000 // 24kbps is a good balance of quality and size for voice
)

type AudioConfig struct {
	SampleRate    int
	Channels      int
	FrameDuration time.Duration // E.g., 20ms
	Bitrate       int

	// Values from 0.0 to 1.0 (or >1.0 for gain)
	InputVolume  float64
	OutputVolume float64

	// If true, zero out samples
	Muted    bool
	Deafened bool
}

// FrameSize returns the number of samples per frame per channel
// e.g. for 48kHz, 20ms -> 48000 * 0.02 = 960 samples
func (c *AudioConfig) FrameSize() int {
	return int(float64(c.SampleRate) * c.FrameDuration.Seconds())
}

// NewDefaultAudioConfig returns a reasonable set of default configs for voice chat
func NewDefaultAudioConfig() *AudioConfig {
	return &AudioConfig{
		SampleRate:    DefaultSampleRate,
		Channels:      DefaultChannels,
		FrameDuration: DefaultFrameDuration,
		Bitrate:       DefaultBitrate,
		InputVolume:   1.0,
		OutputVolume:  1.0,
		Muted:         false,
		Deafened:      false,
	}
}
