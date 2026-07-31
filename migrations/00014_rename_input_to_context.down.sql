UPDATE execution_log
SET context = COALESCE(context -> 'inputs', '{}'::jsonb);

ALTER TABLE execution_log ALTER COLUMN context SET DEFAULT '{}'::jsonb;

ALTER TABLE execution_log RENAME COLUMN context TO input;
