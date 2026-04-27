package audio

import (
	"context"
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

type malgoPlayer struct {
	ctx        context.Context
	malgoCtx   *malgo.AllocatedContext
	device     *malgo.Device
	config     *AudioConfig

	inChan     chan []int16
	
	volMutex   sync.RWMutex
	volume     float64
	deafened   bool
}

func NewMalgoPlayer(ctx context.Context, config *AudioConfig) (AudioPlayer, error) {
	malgoCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {})
	if err != nil {
		return nil, fmt.Errorf("failed to init malgo context: %w", err)
	}

	return &malgoPlayer{
		ctx:      ctx,
		malgoCtx: malgoCtx,
		config:   config,
		inChan:   make(chan []int16, 50), // buffer ~1s audio at 20ms frames
		volume:   config.OutputVolume,
		deafened: config.Deafened,
	}, nil
}

func (p *malgoPlayer) Start() error {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = uint32(p.config.Channels)
	deviceConfig.SampleRate = uint32(p.config.SampleRate)
	deviceConfig.Alsa.NoMMap = 1

	var buffer []int16
	var bufMutex sync.Mutex

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case frame, ok := <-p.inChan:
				if !ok {
					return
				}
				bufMutex.Lock()
				buffer = append(buffer, frame...)
				bufMutex.Unlock()
			}
		}
	}()

	playbackCallbacks := malgo.DeviceCallbacks{
		Data: func(pOutputSample, pInputSamples []byte, framecount uint32) {
			// calculate requested samples from os
			requestedSamples := int(framecount) * p.config.Channels
			
			bufMutex.Lock()
			
			availableSamples := len(buffer)
			samplesToRead := requestedSamples
			if availableSamples < requestedSamples {
				samplesToRead = availableSamples
			}

			samples := make([]int16, requestedSamples)
			if samplesToRead > 0 {
				copy(samples, buffer[:samplesToRead])
				buffer = buffer[samplesToRead:]
			}
			bufMutex.Unlock()

			p.volMutex.RLock()
			vol := p.volume
			deafened := p.deafened
			p.volMutex.RUnlock()

			if deafened {
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

			for i := 0; i < len(samples); i++ {
				pOutputSample[i*2] = byte(samples[i])
				pOutputSample[i*2+1] = byte(samples[i] >> 8)
			}
		},
	}

	device, err := malgo.InitDevice(p.malgoCtx.Context, deviceConfig, playbackCallbacks)
	if err != nil {
		return fmt.Errorf("failed to init playback device: %w", err)
	}
	p.device = device

	err = p.device.Start()
	if err != nil {
		return fmt.Errorf("failed to start playback device: %w", err)
	}

	return nil
}

func (p *malgoPlayer) Stop() error {
	if p.device != nil {
		p.device.Uninit()
		p.device = nil
	}
	if p.malgoCtx != nil {
		_ = p.malgoCtx.Uninit()
		p.malgoCtx.Free()
		p.malgoCtx = nil
	}
	close(p.inChan)
	return nil
}

func (p *malgoPlayer) Write(frame []int16) error {
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case p.inChan <- frame:
		return nil
	default:
		return fmt.Errorf("playback buffer full, frame dropped")
	}
}

func (p *malgoPlayer) SetVolume(v float64) {
	p.volMutex.Lock()
	defer p.volMutex.Unlock()
	p.volume = v
}

func (p *malgoPlayer) SetDeafen(d bool) {
	p.volMutex.Lock()
	defer p.volMutex.Unlock()
	p.deafened = d
}
