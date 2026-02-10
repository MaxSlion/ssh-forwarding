import { ConnectSSH, GetStatus, TestConnection } from '../../wailsjs/go/main/App';
import { main } from '../../wailsjs/go/models';

// Define Frontend-friendly interface with optional fields
export interface FrontendConnectRequest {
    host: string;
    username: string;
    password?: string;
    keyPath?: string;
    agentPath?: string;
}

export type ConnectResponse = main.ConnectResponse;
export type TestConnectionResult = main.TestConnectionResult;

export async function connectV2(req: FrontendConnectRequest): Promise<ConnectResponse> {
    try {
        // Map to Wails struct (fill defaults)
        const wailsReq: main.ConnectRequest = {
            host: req.host,
            username: req.username,
            password: req.password || "",
            keyPath: req.keyPath || "",
            agentPath: req.agentPath || "",
        };
        return await ConnectSSH(wailsReq);
    } catch (e) {
        return { success: false, error: String(e) } as ConnectResponse;
    }
}

export async function testConnection(req: FrontendConnectRequest): Promise<TestConnectionResult> {
    try {
        const wailsReq: main.ConnectRequest = {
            host: req.host,
            username: req.username,
            password: req.password || "",
            keyPath: req.keyPath || "",
            agentPath: req.agentPath || "",
        };
        return await TestConnection(wailsReq);
    } catch (e) {
        return { success: false, error: String(e) } as TestConnectionResult;
    }
}

export async function getStatus(): Promise<{ connected: boolean }> {
    try {
        const connected = await GetStatus();
        return { connected };
    } catch {
        return { connected: false };
    }
}

