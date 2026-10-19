import { Client as MCPClient } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

export type RecordValue = Record<string, any>;
export class Comms {
  private client = new MCPClient({ name: "factory", version: "0.1.0" });
  private connected = false;
  constructor(privateEnv: NodeJS.ProcessEnv = process.env) {
    this.environment = privateEnv;
  }
  private environment: NodeJS.ProcessEnv;
  private connecting?: Promise<void>;
  async connect() {
    if (!this.connecting)
      this.connecting = this.open().catch((error) => {
        this.connecting = undefined;
        throw error;
      });
    return this.connecting;
  }
  private async open() {
    if (this.connected) return;
    const url = new URL(
      this.environment.FACTORY_COMMS_URL || "http://127.0.0.1:8090/mcp",
    );
    if (
      !["http:", "https:"].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    )
      throw new Error("Invalid FACTORY_COMMS_URL");
    const token = this.environment.COMMS_AUTH_TOKEN;
    await this.client.connect(
      new StreamableHTTPClientTransport(url, {
        requestInit: {
          headers: token ? { Authorization: `Bearer ${token}` } : {},
          redirect: "error",
        },
      }),
      { timeout: 10000 },
    );
    this.connected = true;
  }
  async call<T = RecordValue>(
    name: string,
    args: RecordValue = {},
  ): Promise<T> {
    await this.connect();
    const result = await this.client.callTool(
      { name, arguments: args },
      undefined,
      { timeout: 10000 },
    );
    if (result.isError)
      throw new Error(`comms ${name}: ${JSON.stringify(result.content)}`);
    if (!result.structuredContent)
      throw new Error(`comms ${name}: missing structured result`);
    return result.structuredContent as T;
  }
  async pages(
    name: string,
    key: string,
    args: RecordValue,
  ): Promise<RecordValue[]> {
    const values: RecordValue[] = [];
    let after: string | number = name === "list_events" ? 0 : "";
    for (;;) {
      const result = await this.call(name, {
        ...args,
        after,
        limit: name === "list_events" ? 500 : 200,
      });
      const page = result[key] || [];
      values.push(...page);
      if (page.length < (name === "list_events" ? 500 : 200)) return values;
      after = page.at(-1)[name === "list_events" ? "sequence" : "id"];
    }
  }
  async close() {
    this.connected = false;
    this.connecting = undefined;
    await this.client.close();
  }
}
