-- Add action_states column to execution_log for per-action execution state
ALTER TABLE execution_log ADD COLUMN action_states JSONB NOT NULL DEFAULT '{}'::jsonb;
