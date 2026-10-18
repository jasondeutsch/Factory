# Security boundaries

Apply these checks to the change's actual data flow. A warning requires a reachable scenario and impact; a suspicious string in isolation does not prove exploitability.

| Boundary | Questions to answer |
|---|---|
| Authorization | Is the actor permitted to operate on this particular resource, including cross-tenant access? |
| Database | Are untrusted values bound as parameters? Are dynamic identifiers constrained separately? |
| Process execution | Are arguments passed separately, or does a shell interpret user-controlled text? |
| Filesystem | Is access constrained to the intended root, including traversal and symlinks where relevant? |
| HTML | Is untrusted content escaped for its output context? Avoid treating `text/template` as an HTML auto-escaping engine. |
| Outbound HTTP | Can input choose sensitive internal destinations or redirects? What constrains redirects and resource use? |
| Secrets and logs | Can credentials, tokens, or unnecessary personal data reach source, errors, logs, or responses? |

Use the repository's authentication and authorization model. Trace middleware, validation, and call sites before reporting a bypass. Internal communication is not automatically trusted merely because it is inside the network.

Use cryptographic randomness for credentials or security tokens, and suitable established libraries for cryptographic operations. A non-security simulation or randomized game does not need to become a cryptography project.

For inbound data and dependency responses, examine relevant size limits, parsing failures, timeouts, and cleanup. Avoid broad security rewrites unrelated to the change. Identify which protection is missing and how the actual scenario reaches it; do not assign severity from a generic vulnerability name.

Use configured security tools when the task requires them and they are available. Do not install scanners, weaken TLS verification, use live credentials in experiments, or probe external systems without authorization. An unavailable scanner is not evidence of a vulnerability or a clean audit.
