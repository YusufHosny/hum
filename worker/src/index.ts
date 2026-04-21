export interface Env {
	SIGNALING_SERVER: DurableObjectNamespace;
}

export default {
	async fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);

		// fmt -> wss://worker.dev/channelName>?usr=""
		const channelName = url.pathname.split('/')[1];
		const username = url.searchParams.get("usr");

		if (!channelName || !username) {
			return new Response("Missing Channel or User Name", { status: 400 });
		}

		if (request.headers.get("Upgrade") !== "websocket") {
			return new Response("Expected WebSocket", { status: 426 });
		}

		const stub = env.SIGNALING_SERVER.getByName(channelName);
		return stub.fetch(request);
	},
};

export class SignalingServer implements DurableObject {
	constructor(public state: DurableObjectState, public env: Env) {
		this.state.setWebSocketAutoResponse(new WebSocketRequestResponsePair('{"type":"ping"}', '{"type":"pong"}'));
	}

	async fetch(request: Request) {
		const url = new URL(request.url);
		const username = url.searchParams.get('usr')!;

		const allSockets = this.state.getWebSockets();
		for (const socket of allSockets) {
			const [storedUsername] = this.state.getTags(socket);
			if (storedUsername === username) {
				return new Response(JSON.stringify({ message: "User already exists in this channel with this name" }), { status: 401 });
			}
		}

		const webSocketPair = new WebSocketPair();
		const [client, server] = Object.values(webSocketPair);

		this.state.acceptWebSocket(server, [username]);
		this.broadcast({ type: "peer-joined", from: username }, username);

		const memberList = this.state.getWebSockets().map(ws => this.state.getTags(ws)[0]).filter(Boolean);
		server.send(JSON.stringify({ type: "member-list", memberList: memberList }));

		return new Response(null, { status: 101, webSocket: client });
	}

	parseMessage(message: string) {
		let data: any;
		try {
			data = JSON.parse(message);
		} catch (e) {
			throw new Error("Invalid JSON format");
		}

		if (typeof data !== 'object' || data === null) {
			throw new Error("Invalid payload format");
		}

		if (data.type !== undefined && typeof data.type !== 'string') throw new Error("Invalid type field");
		if (data.from !== undefined && typeof data.from !== 'string') throw new Error("Invalid from field");
		if (data.to !== undefined && typeof data.to !== 'string') throw new Error("Invalid to field");

		return data;
	}

	async webSocketMessage(ws: WebSocket, message: string | ArrayBuffer) {
		try {
			if (typeof message !== 'string') {
				ws.send(JSON.stringify({ type: "error", error: "Binary messages not supported" }));
				return;
			}
			const data = this.parseMessage(message as string)
			const [sendername] = this.state.getTags(ws);
			if (data.to) {
				const recipients = this.state.getWebSockets(data.to);

				if (recipients.length === 0)
					throw new Error("Requested recipient not found");
				if (recipients.length > 1)
					throw new Error("Multiple users with this name found (Critical Internal Error)");
				if (data.from !== sendername)
					throw new Error("Message 'from' name does not match sender (Impersonation Protection)");

				return recipients[0].send(JSON.stringify(data));
			}
			else if (data.type === "broadcast") {
				return this.broadcast(data, sendername);
			}
		} catch (e) {
			if (e instanceof Error) {
				ws.send(JSON.stringify({ type: "error", error: e.message }))
			}
		}
	}

	async webSocketClose(ws: WebSocket) {
		const [userId] = this.state.getTags(ws);
		this.broadcast({ type: "peer-left", from: userId }, userId);
	}

	broadcast(message: object, excludedUsername: string) {
		const allSockets = this.state.getWebSockets();
		for (const socket of allSockets) {
			const [storedUsername] = this.state.getTags(socket);
			if (storedUsername !== excludedUsername) {
				socket.send(JSON.stringify(message));
			}
		}
	}
}
