DROP TABLE IF EXISTS flows;
DROP INDEX IF EXISTS idx_flows_slug;

DROP TABLE IF EXISTS execution_log;
DROP INDEX IF EXISTS idx_execution_log_exec_id;
DROP INDEX IF EXISTS idx_execution_log_triggered_by;

DROP TABLE IF EXISTS results;
DROP INDEX IF EXISTS idx_results_uuid;

DROP TRIGGER IF EXISTS new_flow_trigger ON execution_queue;

DROP TABLE IF EXISTS sessions;

DROP TABLE IF EXISTS nodes;
DROP INDEX IF EXISTS idx_nodes_uuid;
DROP INDEX IF EXISTS idx_nodes_name;

DROP TABLE IF EXISTS credentials;
DROP INDEX IF EXISTS idx_credentials_uuid;