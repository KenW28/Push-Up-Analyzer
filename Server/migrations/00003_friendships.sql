-- +goose Up
CREATE TABLE IF NOT EXISTS friendships (
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  friend_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, friend_user_id),
  CHECK (user_id <> friend_user_id)
);

CREATE INDEX IF NOT EXISTS idx_friendships_user_id ON friendships(user_id);
CREATE INDEX IF NOT EXISTS idx_friendships_friend_user_id ON friendships(friend_user_id);

-- +goose Down
DROP TABLE IF EXISTS friendships;
