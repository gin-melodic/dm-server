-- Extend users table to support Supabase Auth fields
ALTER TABLE `users`
  -- auth.users.id, JWT sub field
  ADD COLUMN `supabase_uid` VARCHAR(36) DEFAULT NULL COMMENT 'Supabase Auth user UUID (JWT sub)',
  -- Auth provider type flag
  ADD COLUMN `auth_provider` VARCHAR(20) NOT NULL DEFAULT 'wechat' COMMENT 'Auth provider: wechat | email | ...',
  -- Openid originally NOT NULL, now nullable for email users (no openid)
  MODIFY COLUMN `openid` VARCHAR(128) DEFAULT NULL COMMENT 'WeChat openid, nullable for non-wechat users',
  ADD UNIQUE KEY `uk_supabase_uid` (`supabase_uid`);