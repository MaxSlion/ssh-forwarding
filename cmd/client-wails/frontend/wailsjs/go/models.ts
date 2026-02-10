export namespace main {

	export class ConnectRequest {
		host: string;
		username: string;
		password: string;
		keyPath: string;
		agentPath: string;

		static createFrom(source: any = {}) {
			return new ConnectRequest(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.host = source["host"];
			this.username = source["username"];
			this.password = source["password"];
			this.keyPath = source["keyPath"];
			this.agentPath = source["agentPath"];
		}
	}
	export class ConnectResponse {
		success: boolean;
		error?: string;
		config?: protocol.HandshakeResponse;

		static createFrom(source: any = {}) {
			return new ConnectResponse(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.success = source["success"];
			this.error = source["error"];
			this.config = this.convertValues(source["config"], protocol.HandshakeResponse);
		}

		convertValues(a: any, classs: any, asMap: boolean = false): any {
			if (!a) {
				return a;
			}
			if (a.slice && a.map) {
				return (a as any[]).map(elem => this.convertValues(elem, classs));
			} else if ("object" === typeof a) {
				if (asMap) {
					for (const key of Object.keys(a)) {
						a[key] = new classs(a[key]);
					}
					return a;
				}
				return new classs(a);
			}
			return a;
		}
	}
	export class TestConnectionResult {
		success: boolean;
		error?: string;
		latency?: string;
		sshBanner?: string;

		static createFrom(source: any = {}) {
			return new TestConnectionResult(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.success = source["success"];
			this.error = source["error"];
			this.latency = source["latency"];
			this.sshBanner = source["sshBanner"];
		}
	}
	export class AppSettings {
		agentPath: string;
		connectionTimeout: number;
		localBindAddress: string;
		autoReconnect: boolean;
		theme: string;
		language: string;

		static createFrom(source: any = {}) {
			return new AppSettings(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.agentPath = source["agentPath"] || "./server-agent";
			this.connectionTimeout = source["connectionTimeout"] || 10;
			this.localBindAddress = source["localBindAddress"] || "127.0.0.1";
			this.autoReconnect = source["autoReconnect"] ?? true;
			this.theme = source["theme"] || "light";
			this.language = source["language"] || "zh";
		}
	}

	export class Metrics {
		bytesSent: number;
		bytesReceived: number;

		static createFrom(source: any = {}) {
			return new Metrics(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.bytesSent = source["bytesSent"];
			this.bytesReceived = source["bytesReceived"];
		}
	}

}

export namespace protocol {

	export class PortConfig {
		name: string;
		target: string;
		description?: string;

		static createFrom(source: any = {}) {
			return new PortConfig(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.name = source["name"];
			this.target = source["target"];
			this.description = source["description"];
		}
	}
	export class HandshakeResponse {
		version: string;
		allowed_ports: PortConfig[];
		error?: string;

		static createFrom(source: any = {}) {
			return new HandshakeResponse(source);
		}

		constructor(source: any = {}) {
			if ('string' === typeof source) source = JSON.parse(source);
			this.version = source["version"];
			this.allowed_ports = this.convertValues(source["allowed_ports"], PortConfig);
			this.error = source["error"];
		}

		convertValues(a: any, classs: any, asMap: boolean = false): any {
			if (!a) {
				return a;
			}
			if (a.slice && a.map) {
				return (a as any[]).map(elem => this.convertValues(elem, classs));
			} else if ("object" === typeof a) {
				if (asMap) {
					for (const key of Object.keys(a)) {
						a[key] = new classs(a[key]);
					}
					return a;
				}
				return new classs(a);
			}
			return a;
		}
	}

}

