CREATE TABLE IF NOT EXISTS clawx_sessions (
  id TEXT PRIMARY KEY,
  window_id TEXT NOT NULL DEFAULT '',
  agent_id TEXT NOT NULL DEFAULT '',
  backend TEXT NOT NULL,
  backend_session_id TEXT NOT NULL DEFAULT '',
  conversation_id TEXT NOT NULL,
  cwd TEXT NOT NULL,
  status TEXT NOT NULL,
  lock_token TEXT NOT NULL DEFAULT '',
  last_used_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_clawx_sessions_conversation_last_used
  ON clawx_sessions (conversation_id, last_used_at DESC);
