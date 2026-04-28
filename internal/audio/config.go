package audio

import "time"

const (
	DefaultSampleRate    = 48000
	DefaultChannels      = 1
	DefaultFrameDuration = 20 * time.Millisecond
	DefaultBitrate       = 24000
)

type AudioConfig struct {
	SampleRate    int
	Channels      int
	FrameDuration time.Duration
	Bitrate       int

	InputVolume  float64
	OutputVolume float64

	Muted    bool
	Deafened bool
}

// for 48khz, 20ms frame -> 48k * 0.02 = 960 samples
func (c *AudioConfig) FrameSize() int {
	return int(float64(c.SampleRate) * c.FrameDuration.Seconds())
}

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
