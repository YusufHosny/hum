package p2p

import (
	"time"

	"github.com/YusufHosny/hum/internal/audio"
	"github.com/pion/interceptor/pkg/jitterbuffer"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

func (member *MeshMember) setupOutboundAudio() error {
	audioCapability := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}

	trackLocal, err := webrtc.NewTrackLocalStaticSample(audioCapability, "audio", member.username)
	if err != nil {
		return err
	}
	member.sendTrack = trackLocal

	rtpSender, err := member.connection.AddTrack(trackLocal)
	if err != nil {
		return err
	}

	go member.rtcpSenderLoop(rtpSender)
	return nil
}

func (member *MeshMember) rtcpSenderLoop(rtpSender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		if member.ctx.Err() != nil {
			return
		}
		if _, _, err := rtpSender.Read(buf); err != nil {
			return
		}
	}
}

func (member *MeshMember) rtcpReceiverLoop(rtpReceiver *webrtc.RTPReceiver) {
	buf := make([]byte, 1500)
	for {
		if member.ctx.Err() != nil {
			return
		}
		if _, _, err := rtpReceiver.Read(buf); err != nil {
			return
		}
	}
}

func (member *MeshMember) rtpReceiverLoop(remoteTrack *webrtc.TrackRemote) {
	jb := jitterbuffer.New()

	// Close jitter buffer on exit to prevent goroutine leaks
	defer func() {
		// pion jitterbuffer currently doesn't have a Close method,
		// but stopping the pushes will eventually cause Pop to fail or we handle it via context.
	}()

	go func() {
		for {
			if member.ctx.Err() != nil {
				return
			}
			rtpPacket, _, err := remoteTrack.ReadRTP()
			if err != nil {
				return
			}
			jb.Push(rtpPacket)
		}
	}()

	go func() {
		for {
			if member.ctx.Err() != nil {
				return
			}
			rtpPacket, err := jb.Pop()
			if err != nil {
				return
			}
			if rtpPacket == nil {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			ae := audio.MakeAudioEnvelope(rtpPacket.Payload)
			member.meshContext.acceptAudio(ae)
		}
	}()
}

func (member *MeshMember) sendAudioEnvelope(ae *audio.AudioEnvelope) {
	// 20ms duration for Opus frames
	err := member.sendTrack.WriteSample(media.Sample{Data: ae.Content, Duration: 20 * time.Millisecond})
	if err != nil {
		member.meshContext.Logger().Printf("Failed to send audio packet: %v", err)
	}
}
