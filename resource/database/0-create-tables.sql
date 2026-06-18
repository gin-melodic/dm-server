-- PostgreSQL schema for a fresh Dream Master database.

CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    openid VARCHAR(128) DEFAULT NULL,
    unionid VARCHAR(128) DEFAULT NULL,
    nickname VARCHAR(64) DEFAULT NULL,
    avatar_url VARCHAR(255) DEFAULT NULL,
    mobile VARCHAR(20) DEFAULT NULL,
    email VARCHAR(100) DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    supabase_uid VARCHAR(36) DEFAULT NULL,
    auth_provider VARCHAR(20) NOT NULL DEFAULT 'wechat'
);

COMMENT ON TABLE users IS 'Users table';

COMMENT ON COLUMN users.id IS 'User ID';

COMMENT ON COLUMN users.openid IS 'WeChat openid, nullable for non-wechat users';

COMMENT ON COLUMN users.unionid IS 'WeChat unionid, nullable';

COMMENT ON COLUMN users.nickname IS 'User nickname';

COMMENT ON COLUMN users.avatar_url IS 'Avatar URL';

COMMENT ON COLUMN users.mobile IS 'Mobile number';

COMMENT ON COLUMN users.email IS 'Email address';

COMMENT ON COLUMN users.created_at IS 'Created at';

COMMENT ON COLUMN users.updated_at IS 'Updated at';

COMMENT ON COLUMN users.deleted_at IS 'Deleted at, NULL means not deleted';

COMMENT ON COLUMN users.supabase_uid IS 'Supabase Auth user UUID (JWT sub)';

COMMENT ON COLUMN users.auth_provider IS 'Auth provider: wechat | email | ...';

CREATE TABLE IF NOT EXISTS dreams (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    title VARCHAR(128) DEFAULT NULL,
    content TEXT NOT NULL,
    dream_date DATE NOT NULL,
    tags VARCHAR(255) DEFAULT NULL,
    symbols TEXT DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    emotion VARCHAR(32) NOT NULL DEFAULT 'neutral',
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    confidence_score DECIMAL(5, 4) NOT NULL DEFAULT 0.8600,
    CONSTRAINT fk_dreams_user FOREIGN KEY (user_id) REFERENCES users (id)
);

CREATE INDEX IF NOT EXISTS idx_dreams_user_id ON dreams (user_id);

COMMENT ON TABLE dreams IS 'Dreams table';

COMMENT ON COLUMN dreams.id IS 'Dream ID';

COMMENT ON COLUMN dreams.user_id IS 'Owner user ID';

COMMENT ON COLUMN dreams.title IS 'Dream title';

COMMENT ON COLUMN dreams.content IS 'Dream content';

COMMENT ON COLUMN dreams.dream_date IS 'Date the dream occurred';

COMMENT ON COLUMN dreams.tags IS 'Comma-separated tag list';

COMMENT ON COLUMN dreams.symbols IS 'Comma-separated standard dream symbols';

COMMENT ON COLUMN dreams.created_at IS 'Created at';

COMMENT ON COLUMN dreams.updated_at IS 'Updated at';

COMMENT ON COLUMN dreams.deleted_at IS 'Deleted at, NULL means not deleted';

COMMENT ON COLUMN dreams.status IS 'Dream status: pending/processing/completed/error';

COMMENT ON COLUMN dreams.emotion IS 'Dream emotion selected by user';

COMMENT ON COLUMN dreams.is_favorite IS 'Whether this dream is marked as favorite';

COMMENT ON COLUMN dreams.confidence_score IS 'Dream analysis confidence score';

CREATE TABLE IF NOT EXISTS analysis_sessions (
    id BIGSERIAL PRIMARY KEY,
    dream_id BIGINT NOT NULL,
    session_uuid CHAR(36) NOT NULL,
    analysis_type VARCHAR(32) NOT NULL DEFAULT 'dream',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    progress INT NOT NULL DEFAULT 0,
    result_summary TEXT DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP DEFAULT NULL,
    CONSTRAINT uk_session_uuid UNIQUE (session_uuid),
    CONSTRAINT fk_analysis_sessions_dreams FOREIGN KEY (dream_id) REFERENCES dreams (id)
);

CREATE INDEX IF NOT EXISTS idx_analysis_sessions_dream_id ON analysis_sessions (dream_id);

COMMENT ON TABLE analysis_sessions IS 'Analysis sessions table';

COMMENT ON COLUMN analysis_sessions.id IS 'Session ID';

COMMENT ON COLUMN analysis_sessions.dream_id IS 'Associated dream ID';

COMMENT ON COLUMN analysis_sessions.session_uuid IS 'Analysis session UUID';

COMMENT ON COLUMN analysis_sessions.analysis_type IS 'Analysis type, e.g. psychology';

COMMENT ON COLUMN analysis_sessions.status IS 'Status: pending/processing/completed/error';

COMMENT ON COLUMN analysis_sessions.progress IS 'Progress percentage';

COMMENT ON COLUMN analysis_sessions.result_summary IS 'Analysis summary (stored upon completion)';

COMMENT ON COLUMN analysis_sessions.created_at IS 'Created at';

COMMENT ON COLUMN analysis_sessions.updated_at IS 'Updated at';

COMMENT ON COLUMN analysis_sessions.deleted_at IS 'Deleted at, NULL means not deleted';

CREATE TABLE IF NOT EXISTS user_settings (
    user_id BIGINT NOT NULL,
    language VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
    privacy_mode VARCHAR(32) NOT NULL DEFAULT 'private',
    dream_reminder_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    dream_reminder_time VARCHAR(8) DEFAULT NULL,
    storage_mode VARCHAR(32) NOT NULL DEFAULT 'local_cache',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id),
    CONSTRAINT fk_user_settings_user FOREIGN KEY (user_id) REFERENCES users (id)
);

COMMENT ON TABLE user_settings IS 'User settings table';

COMMENT ON COLUMN user_settings.user_id IS 'Owner user ID';

COMMENT ON COLUMN user_settings.language IS 'Preferred language';

COMMENT ON COLUMN user_settings.privacy_mode IS 'private or cloud_sync';

COMMENT ON COLUMN user_settings.dream_reminder_enabled IS 'Whether dream reminders are enabled';

COMMENT ON COLUMN user_settings.dream_reminder_time IS 'Daily reminder time, HH:mm';

COMMENT ON COLUMN user_settings.storage_mode IS 'local_cache or cloud_sync';

COMMENT ON COLUMN user_settings.created_at IS 'Created at';

COMMENT ON COLUMN user_settings.updated_at IS 'Updated at';

CREATE TABLE IF NOT EXISTS system_configs (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE system_configs IS 'System configuration table';

COMMENT ON COLUMN system_configs.key IS 'Config key';

COMMENT ON COLUMN system_configs.value IS 'Config value (JSON or plain text)';

COMMENT ON COLUMN system_configs.updated_at IS 'Updated at';