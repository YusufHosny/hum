package audio

import (
	"fmt"
	"sync"

	"gopkg.in/hraban/opus.v2"
)

type opusEncoder struct {
	enc        *opus.Encoder
	dec        *opus.Decoder
	config     *AudioConfig
	bufferSize int
	encodeMux  sync.Mutex
	decodeMux  sync.Mutex
}

func NewOpusEncoder(cfg *AudioConfig) (AudioEncoder, error) {
	enc, err := opus.NewEncoder(cfg.SampleRate, cfg.Channels, opus.AppVoIP)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}

	err = enc.SetBitrate(cfg.Bitrate)
	if err != nil {
		return nil, fmt.Errorf("failed to set opus bitrate: %w", err)
	}

	dec, err := opus.NewDecoder(cfg.SampleRate, cfg.Channels)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus decoder: %w", err)
	}

	return &opusEncoder{
		enc:        enc,
		dec:        dec,
		config:     cfg,
		bufferSize: 1024 * 4,
	}, nil
}

func (o *opusEncoder) Encode(pcm []int16) ([]byte, error) {
	o.encodeMux.Lock()
	defer o.encodeMux.Unlock()

	outData := make([]byte, o.bufferSize)
	n, err := o.enc.Encode(pcm, outData)
	if err != nil {
		return nil, fmt.Errorf("encode failed: %w", err)
	}
	return outData[:n], nil
}

func (o *opusEncoder) Decode(encoded []byte) ([]int16, error) {
	o.decodeMux.Lock()
	defer o.decodeMux.Unlock()

	outData := make([]int16, o.config.FrameSize()*o.config.Channels)
	
	n, err := o.dec.Decode(encoded, outData)
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return outData[:n*o.config.Channels], nil
}

func (o *opusEncoder) SetBitrate(bitrate int) error {
	o.encodeMux.Lock()
	defer o.encodeMux.Unlock()
	
	o.config.Bitrate = bitrate
	return o.enc.SetBitrate(bitrate)
}
