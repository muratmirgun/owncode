-- +goose Up
CREATE TABLE turn_checkpoints (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
 state TEXT NOT NULL,
 before_state BLOB NOT NULL,
 after_state BLOB
);
CREATE TABLE hidden_messages (
 message_id TEXT PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE
);
-- +goose Down
DROP TABLE hidden_messages;
DROP TABLE turn_checkpoints;
