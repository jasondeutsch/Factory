---
name: coordination
description: Coordinate a bounded task across agents using explicit ownership, messages, decisions, and evidence when a communication service is available.
---

# Coordinate agent work

Establish the user's task, project, available agents, and decision authority before assigning work. Keep one owner for each piece of work. Give an agent the expected output, relevant context, and a clear handoff point. Start only agents needed for the task.

If Comms is connected, use its task, participant, assignment, inbox, event, and report tools as the shared record. Register only actual running participants. Read the current record before retrying a write or resuming an interrupted assignment. A sent message, a running session, and a submitted result are different facts. Check the recipient's read/exec state and the assignment result before claiming progress or completion.

Bring material questions, review findings, blocked work, and approval requests to the user. Record decisions with their actual scope and evidence. Never infer approval from silence. Keep the agents' work visible and report the current owner, next action, and any known limitation.

Without a connected communication service, coordinate directly in the user's conversation and state which handoffs are not durable or automatically delivered. Do not invent task records or claim to have contacted another agent.
