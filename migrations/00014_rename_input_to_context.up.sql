ALTER TABLE execution_log RENAME COLUMN input TO context;

ALTER TABLE execution_log ALTER COLUMN context SET DEFAULT '{"inputs": {}, "outputs": {}}'::jsonb;

UPDATE execution_log
SET context = jsonb_build_object('inputs', context, 'outputs', '{}'::jsonb);
