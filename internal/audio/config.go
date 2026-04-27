package audio

import "time"

const (
	DefaultSampleRate    = 48000 // 48khz standard for opus voip
	DefaultChannels      = 1     // mono audio enough for voice
	DefaultFrameDuration = 20 * time.Millisecond // 20ms frame is opus standard
	DefaultBitrate       = 24000 // 24kbps good balance for voice
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
