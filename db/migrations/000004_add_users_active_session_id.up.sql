ALTER TABLE users
ADD COLUMN active_session_id UUID REFERENCES sessions (id) ON DELETE SET NULL;