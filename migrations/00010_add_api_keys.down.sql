-- Rollback API Keys table
DROP INDEX IF EXISTS idx_api_keys_key_prefix;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_uuid;
DROP TABLE IF EXISTS api_keys;
