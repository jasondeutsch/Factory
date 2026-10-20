# Framework tools and tradeoffs

Factory supplies coordination infrastructure; teams supply agents and factory configurations in a separate repository. No application language, model, review policy or third-party agent plugin is selected by this framework.

| Component | Purpose | Tradeoff |
| --- | --- | --- |
| Go comms service and SQLite | Durable tasks, assignments, messages, decisions, usage and reports through MCP. | Requires a reachable service. The optional shared bearer token does not authenticate individual agents. |
| Compose and SQLite Web | Deploy comms with a read-only database admin panel. | Requires Docker/Compose. The named volume persists until explicitly removed. |
| TypeScript launchers and official MCP SDK | Resolve factory configuration, prepare private runner profiles, attach documents and dispatch visible sessions. | Requires Node.js 22.18+ and locked npm dependencies. No container sandbox or workflow scheduler. |
| Pi adapter and delivery extension | Apply native configuration and queue inbox hints outside model turns. | Pi-specific integration behind a documented runner contract; reconnect can repeat hints. A stopped process cannot be awakened. |
| Herdr, Terminal, iTerm adapters | Open visible sessions for feedback. | Herdr requires its host context. Terminal/iTerm currently support macOS and require appropriate shell environment settings. |
| Terminal monitor (Bubble Tea and Lip Gloss) | Read-only task overview, agent contact, message routes and activity in a terminal. | Requires running Comms and Go to launch from source, or a prebuilt binary. Polls every three seconds; contact is not progress. |
| Browser monitor and HTML reports | Inspect recorded status and evidence without a terminal client. | Bounded detail views; reports refresh on request. Unknown measurements remain unknown. |
| Optional Herdr fallback bridge | Notify runners lacking a native delivery adapter. | Captured bindings and conservative recovery are required; never combine with Pi's native delivery for one recipient. |

The coordinator chooses work from the team's recipe and communicates through comms. Delivery does not choose stages, grant approvals, or establish correct results. A member's instruction/skill selection is explicit in `factory.json`; merely installing a skill does not connect a tool.

The framework's task helper scaffolds only the `artifacts` declared in a selected factory. Its optional loop launch uses member configuration and actual artifact approvals; the selected extension owns iteration behavior. Application checks, ticket tools, model choices, prompts and review methods belong to the tools repository.

See [configuration](configuration.md), [startup](../orchestrator/README.md), and the [delivery contract](../orchestrator/comms/delivery.md) for the operational details. Tests verify protocol and launch boundaries; live model behavior and terminal integration need their own evaluation.
