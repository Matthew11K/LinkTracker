DROP INDEX IF EXISTS idx_chats_notification_mode;

ALTER TABLE chats
DROP COLUMN IF EXISTS notification_mode,
DROP COLUMN IF EXISTS digest_time; 