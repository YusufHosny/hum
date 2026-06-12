# Hum

Encrypted peer-to-peer voice and text chat that lives in your terminal.

Hum connects peers directly over a WebRTC mesh — audio and messages flow between
participants, not through a central server. Everything is end-to-end encrypted
with a key derived from the channel name and a shared passkey, so the signaling
server (and anyone in between) only ever sees ciphertext.

## Features

- **Text chat** over WebRTC data channels
- **Voice calls** with Opus audio and voice-activity detection
- **Double-locked encryption** — Hum's own end-to-end layer (XChaCha20-Poly1305, key derived via Argon2id) on top of WebRTC's built-in transport security (DTLS / DTLS-SRTP)
- **Full-mesh P2P** — every peer connects directly to every other peer
- **Terminal UI** built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)

## Install

Grab a prebuilt binary from the [latest release](https://github.com/YusufHosny/hum/releases/latest):

- **Windows** — download `Hum_Windows.exe` and run it. It's statically linked, no dependencies.
- **Linux (Debian/Ubuntu)** — download `Hum_Debian`, then install the Opus runtime libraries:
  ```sh
  sudo apt install libopus0 libopusfile0
  chmod +x Hum_Debian
  ./Hum_Debian
  ```
  Audio playback/capture also needs PulseAudio or ALSA.

## Usage

Launch the binary. On first run, enter a username. Then, from the home screen:

1. Type a **channel name** and a **passkey**.
2. Select **Connect**.

Anyone who joins the same channel with the same passkey lands in the same room.
The passkey never leaves your machine — it's mixed with the channel name to derive
the encryption key locally, so peers with the wrong passkey simply can't decrypt
anything.

You can also pass flags to override the stored config:

```sh
hum -u alice                 # set username
hum -s wss://your-worker.dev # override the signaling server
```

### Controls

The message box is focused by default — just type and press `Enter` to send.
Press `Tab` / `Shift+Tab` to move between the action buttons:

| Action | How |
|--------|-----|
| Send a message | type, then `Enter` |
| Join / leave the voice call | `Tab` to **Join Call**, `Enter` |
| Mute / unmute mic | `Tab` to **Mute**, `Enter` |
| Deafen (mute everyone) | `Tab` to **Deafen**, `Enter` |
| Leave the channel | `Esc` |

## Configuration

Config lives at `<user-config-dir>/hum/config.json`:

- Linux: `~/.config/hum/config.json`
- Windows: `%AppData%\hum\config.json`
- macOS: `~/Library/Application Support/hum/config.json`

```json
{
  "username": "alice",
  "signalingUrl": "wss://hum-signaling-worker.example.workers.dev",
  "stunServers": ["stun:stun.l.google.com:19302"],
  "inputVolume": 1.0,
  "outputVolume": 1.0,
  "voiceThreshold": 10,
  "recentChannels": []
}
```

`voiceThreshold` is a **1–100 sensitivity knob** (lower = more sensitive, transmits
quieter sound; higher = only louder speech opens the gate). It's also editable from
the in-app **Settings** screen.

## Building from source

Requires **Go 1.26.2+**. The audio stack uses cgo, so you also need a C compiler
and the Opus libraries.

```sh
# Linux build deps
sudo apt install pkg-config libopus-dev libopusfile-dev

# build for the current platform
CGO_ENABLED=1 go build -o bin/hum ./cmd/hum
```

A helper script builds for both Linux and Windows:

```sh
./build.sh            # both
./build.sh linux      # linux only
./build.sh windows    # windows only
```

Cross-compiling the Windows binary additionally needs the mingw toolchain
(`gcc-mingw-w64-x86-64`) and Opus built for `x86_64-w64-mingw32`; see the comments
in `build.sh`.

## Signaling server

Peers find each other through a small [Cloudflare Worker](worker/) backed by a
Durable Object that relays WebRTC offers, answers, and ICE candidates (it never
sees decrypted content). Deploy your own with [Wrangler](https://developers.cloudflare.com/workers/wrangler/):

```sh
cd worker
wrangler deploy
```

Then point `signalingUrl` in your config at the resulting `wss://…workers.dev` URL.

## How it works

- **Signaling** — a peer opens a WebSocket to `wss://<server>/<channel>?usr=<name>`.
  The Durable Object tracks who's in the channel and relays signaling messages
  between them.
- **Connection** — peers exchange SDP offers/answers and ICE candidates, then
  establish direct WebRTC connections (data channels for chat, a media track for
  audio).
- **Encryption (two layers)** — WebRTC already encrypts every connection at the
  transport level: data channels run over DTLS, the audio track over DTLS-SRTP,
  and signaling over a `wss://` (TLS) WebSocket. On top of that, Hum adds its own
  end-to-end layer — the channel name (salt) + passkey are run through Argon2id to
  derive a 256-bit key, and every chat and audio payload is sealed with
  XChaCha20-Poly1305 before it's handed to WebRTC. So traffic is effectively
  double-locked, and because the passkey-derived key never leaves your machine,
  even the signaling server (which brokers the DTLS handshake) can't read it.
- **Audio** — captured at 48 kHz mono, gated by voice activity, encoded with Opus,
  encrypted, and sent over the peer connection.

## Limitations

- **STUN only, no TURN.** Connections work on most home and LAN networks, but two
  peers behind symmetric NATs or strict firewalls may fail to connect, since there's
  no relay fallback.
- Peers must share the channel **and** passkey out-of-band — there's no key exchange.
