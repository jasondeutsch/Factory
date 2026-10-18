"""Small client for this service's stateless JSON Streamable HTTP endpoint.

Used by artifact/session helpers; agents call the same tools through native MCP.
No database or alternative task store lives in this client.
"""
from __future__ import annotations

import json
import os
import uuid
from urllib.error import HTTPError, URLError
from urllib.parse import urlsplit
from urllib.request import Request, build_opener, HTTPRedirectHandler

DEFAULT_URL = "http://127.0.0.1:8090/mcp"


class NoRedirects(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None  # Never forward a service token to a different endpoint.


class Client:
    def __init__(self, environment=None):
        environment = os.environ if environment is None else environment
        self.url = environment.get("FACTORY_COMMS_URL", DEFAULT_URL)
        parsed = urlsplit(self.url)
        if parsed.scheme not in ("http", "https") or not parsed.netloc or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise ValueError("FACTORY_COMMS_URL must be an HTTP(S) endpoint without credentials, query, or fragment")
        self.token = environment.get("COMMS_AUTH_TOKEN", "")
        self.transport = build_opener(NoRedirects())
        self.protocol = "2025-03-26"
        initialized = self.request("initialize", {
            "protocolVersion": self.protocol, "capabilities": {},
            "clientInfo": {"name": "factory-helpers", "version": "0.1.0"},
        })
        self.protocol = initialized["protocolVersion"]
        self.request("notifications/initialized", notification=True)

    def request(self, method, params=None, notification=False):
        payload = {"jsonrpc": "2.0", "method": method}
        if params is not None:
            payload["params"] = params
        if not notification:
            payload["id"] = uuid.uuid4().hex
        headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream",
                   "MCP-Protocol-Version": self.protocol}
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        request = Request(self.url, data=json.dumps(payload).encode(), headers=headers, method="POST")
        try:
            with self.transport.open(request, timeout=10) as response:
                if notification:
                    return None
                result = json.load(response)
        except HTTPError as error:
            raise ValueError(f"comms HTTP {error.code}; check the service endpoint and COMMS_AUTH_TOKEN") from None
        except (URLError, TimeoutError, OSError):
            raise ValueError("comms unavailable; check Compose and FACTORY_COMMS_URL; no local task-state fallback") from None
        except (ValueError, UnicodeError):
            raise ValueError("comms returned an invalid JSON response") from None
        if result.get("id") != payload["id"] or "result" not in result:
            raise ValueError("comms rejected the MCP request")
        return result["result"]

    def call(self, tool_name, **arguments):
        result = self.request("tools/call", {"name": tool_name, "arguments": arguments})
        if result.get("isError"):
            # The service's tool errors contain validation details, not raw HTTP diagnostics.
            message = " ".join(item.get("text", "") for item in result.get("content", []) if item.get("type") == "text")
            raise ValueError(f"comms {tool_name}: {message}")
        output = result.get("structuredContent")
        if not isinstance(output, dict):
            raise ValueError(f"comms {tool_name}: missing structured result")
        return output

    def events(self, task_id):
        cursor = 0
        while True:
            page = self.call("list_events", task_id=task_id, after=cursor, limit=500)["events"] or []
            yield from page
            if len(page) < 500:
                return
            cursor = page[-1]["sequence"]

    def inbox(self, task_id, recipient, include_acknowledged=False):
        cursor = ""
        while True:
            page = self.call("read_inbox", task_id=task_id, recipient=recipient,
                             include_acknowledged=include_acknowledged, after=cursor, limit=200)["messages"] or []
            yield from page
            if len(page) < 200:
                return
            cursor = page[-1]["id"]
