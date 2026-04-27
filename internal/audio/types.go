package audio

// AudioRecorder captures audio from the system microphone.
type AudioRecorder interface {
	Start() error
	Stop() error
	// Read blocks until a frame is ready. It returns a slice of PCM samples.
	Read() ([]int16, error)
	SetVolume(float64)
	SetMute(bool)
}

// AudioPlayer plays audio to the system speakers.
type AudioPlayer interface {
	Start() error
	Stop() error
	// Write enqueues a frame of PCM samples for playback.
	Write([]int16) error
	SetVolume(float64)
	SetDeafen(bool)
}

// AudioEncoder handles compression/decompression of PCM audio.
type AudioEncoder interface {
	Encode(pcm []int16) ([]byte, error)
	Decode(encoded []byte) ([]int16, error)
	SetBitrate(int) error
}
