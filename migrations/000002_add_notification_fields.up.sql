ALTER TABLE chats
ADD COLUMN notification_mode VARCHAR(10) NOT NULL DEFAULT 'instant',
ADD COLUMN digest_time TIME NOT NULL DEFAULT '10:00:00';

CREATE INDEX IF NOT EXISTS idx_chats_notification_mode ON chats(notification_mode); 