package audio

import (
	"context"
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

type malgoRecorder struct {
	ctx        context.Context
	malgoCtx   *malgo.AllocatedContext
	device     *malgo.Device
	config     *AudioConfig
	
	outChan    chan []int16
	
	volMutex   sync.RWMutex
	volume     float64
	muted      bool
}

func NewMalgoRecorder(ctx context.Context, config *AudioConfig) (AudioRecorder, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {})
	if err != nil {
		return nil, fmt.Errorf("failed to init malgo context: %w", err)
	}

	return &malgoRecorder{
		ctx:      ctx,
		malgoCtx: malgoCtx,
		config:   config,
		outChan:  make(chan []int16, 10),
		volume:   config.InputVolume,
		muted:    config.Muted,
	}, nil
}

func (r *malgoRecorder) Start() error {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(r.config.Channels)
	deviceConfig.SampleRate = uint32(r.config.SampleRate)
	deviceConfig.Alsa.NoMMap = 1

	frameSize := r.config.FrameSize() * r.config.Channels
	buffer := make([]int16, 0, frameSize)

	captureCallbacks := malgo.DeviceCallbacks{
		Data: func(pOutputSample, pInputSamples []byte, framecount uint32) {
			// convert malgo s16 bytes to int16
			samples := make([]int16, len(pInputSamples)/2)
			for i := 0; i < len(samples); i++ {
				samples[i] = int16(pInputSamples[i*2]) | (int16(pInputSamples[i*2+1]) << 8)
			}

			r.volMutex.RLock()
			vol := r.volume
			muted := r.muted
			r.volMutex.RUnlock()

			if muted {
				for i := range samples {
					samples[i] = 0
				}
			} else if vol != 1.0 {
				for i := range samples {
					val := float64(samples[i]) * vol
					if val > 32767 {
						val = 32767
					} else if val < -32768 {
						val = -32768
					}
					samples[i] = int16(val)
				}
			}

			buffer = append(buffer, samples...)

			for len(buffer) >= frameSize {
				frame := make([]int16, frameSize)
				copy(frame, buffer[:frameSize])
				
				select {
				case r.outChan <- frame:
				default:
				}

				buffer = buffer[frameSize:]
			}
		},
	}

	device, err := malgo.InitDevice(r.malgoCtx.Context, deviceConfig, captureCallbacks)
	if err != nil {
		return fmt.Errorf("failed to init capture device: %w", err)
	}
	r.device = device

	err = r.device.Start()
	if err != nil {
		return fmt.Errorf("failed to start capture device: %w", err)
	}

	return nil
}

func (r *malgoRecorder) Stop() error {
	if r.device != nil {
		r.device.Uninit()
		r.device = nil
	}
	if r.malgoCtx != nil {
		_ = r.malgoCtx.Uninit()
		r.malgoCtx.Free()
		r.malgoCtx = nil
	}
	close(r.outChan)
	return nil
}

func (r *malgoRecorder) Read() ([]int16, error) {
	select {
	case <-r.ctx.Done():
		return nil, r.ctx.Err()
	case frame, ok := <-r.outChan:
		if !ok {
			return nil, fmt.Errorf("recorder stopped")
		}
		return frame, nil
	}
}

func (r *malgoRecorder) SetVolume(v float64) {
	r.volMutex.Lock()
	defer r.volMutex.Unlock()
	r.volume = v
}

func (r *malgoRecorder) SetMute(m bool) {
	r.volMutex.Lock()
	defer r.volMutex.Unlock()
	r.muted = m
}
