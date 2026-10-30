CREATE TABLE account (
  id            INTEGER PRIMARY KEY CHECK (id = 1),
  username      TEXT    NOT NULL,
  password_hash TEXT    NOT NULL,
  version       INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
  token_hash      BLOB    PRIMARY KEY,
  account_version INTEGER NOT NULL,
  created_at      INTEGER NOT NULL,
  expires_at      INTEGER NOT NULL
);

CREATE TABLE events (
  id         TEXT    PRIMARY KEY,
  title      TEXT    NOT NULL,
  memo       TEXT    NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE candidates (
  id       INTEGER PRIMARY KEY,
  event_id TEXT    NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  label    TEXT    NOT NULL
);

CREATE TABLE answers (
  id         INTEGER PRIMARY KEY,
  event_id   TEXT    NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  name       TEXT    NOT NULL,
  comment    TEXT    NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE choices (
  answer_id    INTEGER NOT NULL REFERENCES answers(id) ON DELETE CASCADE,
  candidate_id INTEGER NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  value        INTEGER NOT NULL CHECK (value IN (0, 1, 2)),
  PRIMARY KEY (answer_id, candidate_id)
);

CREATE INDEX candidates_event ON candidates(event_id, position);
CREATE INDEX answers_event ON answers(event_id, id);
