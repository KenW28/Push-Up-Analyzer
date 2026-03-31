-- +goose Up
CREATE TABLE IF NOT EXISTS friend_requests (
  id BIGSERIAL PRIMARY KEY,
  sender_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  receiver_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'denied', 'cancelled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  responded_at TIMESTAMPTZ NULL,
  CHECK (sender_user_id <> receiver_user_id)
);

CREATE INDEX IF NOT EXISTS idx_friend_requests_sender_status ON friend_requests(sender_user_id, status);
CREATE INDEX IF NOT EXISTS idx_friend_requests_receiver_status ON friend_requests(receiver_user_id, status);

CREATE UNIQUE INDEX IF NOT EXISTS idx_friend_requests_unique_pending_pair
ON friend_requests (LEAST(sender_user_id, receiver_user_id), GREATEST(sender_user_id, receiver_user_id))
WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS friend_requests;
