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
PRAGMA user_version = 1;
