CREATE TABLE IF NOT EXISTS tasks (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 id TEXT NOT NULL UNIQUE,
 project TEXT NOT NULL,
 name TEXT NOT NULL,
 brief TEXT NOT NULL,
 owner TEXT NOT NULL,
 status TEXT NOT NULL,
 summary TEXT NOT NULL,
 version INTEGER NOT NULL DEFAULT 1,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 request_key TEXT,
 UNIQUE(project, request_key)
);
CREATE INDEX IF NOT EXISTS tasks_project ON tasks(project, sequence);
CREATE TABLE IF NOT EXISTS messages (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 sender TEXT NOT NULL,
 recipient TEXT NOT NULL,
 kind TEXT NOT NULL,
 text TEXT NOT NULL,
 created_at TEXT NOT NULL,
 acknowledged_at TEXT,
 request_key TEXT,
 UNIQUE(task_id, sender, request_key)
);
CREATE INDEX IF NOT EXISTS messages_inbox ON messages(task_id, recipient, id);
CREATE TABLE IF NOT EXISTS events (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 actor TEXT NOT NULL,
 kind TEXT NOT NULL,
 stage TEXT NOT NULL DEFAULT '',
 attempt INTEGER NOT NULL DEFAULT 0,
 message TEXT NOT NULL,
 data TEXT NOT NULL DEFAULT '{}',
 created_at TEXT NOT NULL,
 request_key TEXT,
 UNIQUE(task_id, actor, request_key)
);
CREATE INDEX IF NOT EXISTS events_task ON events(task_id, sequence);
CREATE TABLE IF NOT EXISTS participants (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 instance_id TEXT NOT NULL,
 role TEXT NOT NULL,
 routing_identity TEXT NOT NULL,
 session_ref TEXT NOT NULL,
 registered_at TEXT NOT NULL,
 retired_at TEXT,
 UNIQUE(id, task_id),
 UNIQUE(task_id, instance_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS participants_active_routing ON participants(task_id, routing_identity) WHERE retired_at IS NULL;
CREATE TABLE IF NOT EXISTS assignments (
 id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id),
 assigning_participant_id TEXT NOT NULL,
 assignee_id TEXT NOT NULL,
 title TEXT NOT NULL,
 instructions TEXT NOT NULL,
 inputs TEXT NOT NULL,
 expected_output TEXT NOT NULL,
 stage TEXT NOT NULL,
 attempt INTEGER NOT NULL CHECK(attempt > 0),
 state TEXT NOT NULL CHECK(state IN ('offered','accepted','working','blocked','needs_input','submitted','declined','cancelled')),
 summary TEXT NOT NULL,
 version INTEGER NOT NULL CHECK(version > 0),
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 result TEXT,
 FOREIGN KEY(assigning_participant_id, task_id) REFERENCES participants(id, task_id),
 FOREIGN KEY(assignee_id, task_id) REFERENCES participants(id, task_id)
);
CREATE INDEX IF NOT EXISTS assignments_task ON assignments(task_id, id);
CREATE INDEX IF NOT EXISTS assignments_assignee ON assignments(task_id, assignee_id, state);
CREATE TABLE IF NOT EXISTS coordination_requests (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 actor_key TEXT NOT NULL,
 request_key TEXT NOT NULL,
 operation TEXT NOT NULL,
 input TEXT NOT NULL,
 output TEXT NOT NULL,
 PRIMARY KEY(task_id, actor_key, request_key)
);

-- Host notification state is separate from reading and assignment ownership.
CREATE TABLE IF NOT EXISTS deliveries (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 message_id TEXT NOT NULL REFERENCES messages(id),
 participant_id TEXT NOT NULL REFERENCES participants(id),
 state TEXT NOT NULL CHECK(state IN ('claimed','submitted','retry','failed','uncertain')),
 attempts INTEGER NOT NULL,
 detail TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 retry_at INTEGER NOT NULL,
 lease_token TEXT NOT NULL,
 PRIMARY KEY(message_id,participant_id)
);
CREATE INDEX IF NOT EXISTS delivery_participant ON deliveries(task_id,participant_id,state);

CREATE TABLE IF NOT EXISTS sessions (
 participant_id TEXT PRIMARY KEY REFERENCES participants(id),
 manifest TEXT NOT NULL,
 contact_state TEXT NOT NULL DEFAULT 'unknown',
 observed_at TEXT NOT NULL DEFAULT '',
 expires_at TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS task_factories (
 task_id TEXT PRIMARY KEY REFERENCES tasks(id),
 snapshot TEXT NOT NULL
);
