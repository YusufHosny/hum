package audio

type AudioRecorder interface {
	Start() error
	Stop() error
	Read() ([]int16, error)
	SetVolume(float64)
	SetMute(bool)
}

type AudioPlayer interface {
	Start() error
	Stop() error
	Write([]int16) error
	SetVolume(float64)
	SetDeafen(bool)
}

type AudioEncoder interface {
	Encode(pcm []int16) ([]byte, error)
	Decode(encoded []byte) ([]int16, error)
	SetBitrate(int) error
}
