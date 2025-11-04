package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/cvhariharan/flowctl/internal/scheduler"
	"github.com/google/uuid"
)

var (
	ErrFlowNotFound = errors.New("flow not found")
)

// detectFlowFormat determines the flow format based on file extension
func detectFlowFormat(filePath string) models.FlowFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".huml":
		return models.FlowFormatHUML
	case ".yaml", ".yml":
		return models.FlowFormatYAML
	default:
		return models.FlowFormatYAML // Default to YAML for backwards compatibility
	}
}

// isFlowFile checks if the file has a valid flow extension (.yaml, .yml, or .huml)
func isFlowFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".huml")
}

func (c *Core) GetFlowByID(id string, namespaceID string) (models.Flow, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	f, ok := c.flows[fmt.Sprintf("%s:%s", id, namespaceID)]
	if !ok {
		return models.Flow{}, ErrFlowNotFound
	}

	return f, nil
}

func (c *Core) GetAllFlows(ctx context.Context, namespaceID string) ([]models.Flow, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	flows, err := c.store.GetFlowsByNamespace(ctx, namespaceUUID)
	if err != nil {
		return nil, fmt.Errorf("could not get flows for namespace %s: %w", namespaceID, err)
	}

	var fs []models.Flow
	for _, v := range flows {
		fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
	}
	return fs, nil
}

func (c *Core) GetFlowsPaginated(ctx context.Context, namespaceID string, limit, offset int) ([]models.Flow, int64, int64, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	flows, err := c.store.ListFlowsPaginated(ctx, repo.ListFlowsPaginatedParams{
		Uuid:   namespaceUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get paginated flows for namespace %s: %w", namespaceID, err)
	}

	var fs []models.Flow
	var pageCount, totalCount int64

	for _, v := range flows {
		fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return fs, pageCount, totalCount, nil
}

func (c *Core) SearchFlows(ctx context.Context, namespaceID string, query string, limit, offset int) ([]models.Flow, int64, int64, error) {
	c.rwf.RLock()
	defer c.rwf.RUnlock()
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	flows, err := c.store.SearchFlowsPaginated(ctx, repo.SearchFlowsPaginatedParams{
		Uuid:    namespaceUUID,
		Column2: query,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not search flows for namespace %s: %w", namespaceID, err)
	}

	var fs []models.Flow
	var pageCount, totalCount int64

	for _, v := range flows {
		fs = append(fs, c.flows[fmt.Sprintf("%s:%s", v.Slug, namespaceID)])
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return fs, pageCount, totalCount, nil
}

func (c *Core) GetFlowFromLogID(logID string, namespaceID string) (models.Flow, error) {
	f, ok := c.logMap[logID]
	if !ok {
		namespaceUUID, err := uuid.Parse(namespaceID)
		if err != nil {
			return models.Flow{}, fmt.Errorf("invalid namespace UUID: %w", err)
		}
		df, err := c.store.GetFlowFromExecID(context.Background(), repo.GetFlowFromExecIDParams{
			ExecID: logID,
			Uuid:   namespaceUUID,
		})
		if err != nil {
			return models.Flow{}, fmt.Errorf("could not get flow for exec id %s: %w", logID, err)
		}
		return c.GetFlowByID(df.Slug, namespaceID)
	}

	return c.GetFlowByID(f, namespaceID)
}

// QueueFlowExecution adds a flow in the execution queue. The ID returned is the execution queue ID.
// Exec ID should be universally unique, this is used to create the log stream and identify each execution
func (c *Core) QueueFlowExecution(ctx context.Context, f models.Flow, input map[string]interface{}, userUUID string, namespaceID string) (string, error) {
	if !f.Meta.AllowOverlap {
		namespaceUUID, err := uuid.Parse(namespaceID)
		if err != nil {
			return "", fmt.Errorf("invalid namespace UUID: %w", err)
		}
		execExists, err := c.store.ExecutionExistsForFlow(ctx, repo.ExecutionExistsForFlowParams{
			Slug: f.Meta.ID,
			Uuid: namespaceUUID,
		})
		if err != nil {
			return "", fmt.Errorf("error checking existing executions for flow %s: %w", f.Meta.ID, err)
		}
		if execExists {
			return "", fmt.Errorf("could not queue flow %s for execution: execution overlap is disabled", f.Meta.Name)
		}
	}

	info, err := c.queueFlow(ctx, f, input, "", 0, userUUID, namespaceID)
	if err != nil {
		return "", err
	}

	return info, nil
}

// ResumeFlowExecution moves the task to a resume queue for further processing.
func (c *Core) ResumeFlowExecution(ctx context.Context, execID string, actionID string, userUUID string, namespaceID string) error {
	exec, err := c.GetExecutionByExecID(ctx, execID, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get exec %s: %w", execID, err)
	}

	f, err := c.GetFlowFromLogID(execID, namespaceID)
	if err != nil {
		return err
	}

	actionIndex, err := f.GetActionIndexByID(actionID)
	if err != nil {
		return err
	}

	if _, err := c.queueFlow(ctx, f, exec.Input, execID, actionIndex, userUUID, namespaceID); err != nil {
		return err
	}

	return nil
}

// GetNodesByNames retrieves nodes by their names and returns a slice of models.Node
// This is used as a lookup function for converting flows to task models
func (c *Core) GetNodesByNames(ctx context.Context, nodeNames []string, namespaceUUID uuid.UUID) ([]models.Node, error) {
	if len(nodeNames) == 0 {
		return nil, nil
	}

	n, err := c.store.GetNodesByNames(ctx, repo.GetNodesByNamesParams{
		Column1: nodeNames,
		Uuid:    namespaceUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get nodes by names %v: %w", nodeNames, err)
	}

	var nodes []models.Node
	for _, v := range n {
		key := v.CredentialKeyData.String

		// decrypt the key
		dKey, err := hex.DecodeString(key)
		if err != nil {
			return nil, fmt.Errorf("could not decode key for node %s: %w", v.Name, err)
		}

		decryptedKey, err := c.keeper.Decrypt(ctx, []byte(dKey))
		if err != nil {
			return nil, fmt.Errorf("could not decrypt key for node %s: %w", v.Name, err)
		}

		nodes = append(nodes, models.Node{
			ID:             v.Uuid.String(),
			Name:           v.Name,
			Hostname:       v.Hostname,
			Port:           int(v.Port),
			Username:       v.Username,
			OSFamily:       v.OsFamily,
			Tags:           v.Tags,
			ConnectionType: string(v.ConnectionType),
			Auth: models.NodeAuth{
				CredentialID: v.CredentialUuid.UUID.String(),
				Method:       models.AuthMethod(v.AuthMethod),
				Key:          string(decryptedKey),
			},
		})
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found for names %v", nodeNames)
	}

	return nodes, nil
}

// queueFlow adds a flow to the execution queue. If the actionIndex is not zero, it is moved to a resume queue.
func (c *Core) queueFlow(ctx context.Context, f models.Flow, input map[string]interface{}, execID string, actionIndex int, userUUID string, namespaceID string) (string, error) {
	// If execID is empty, it is a new flow execution
	if execID == "" {
		execID = uuid.NewString()
	}

	userID, err := uuid.Parse(userUUID)
	if err != nil {
		return "", fmt.Errorf("user id is not a UUID: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return "", fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Convert to scheduler flow format
	schedulerFlow, err := models.ConvertToSchedulerFlow(ctx, f, namespaceUUID, c.GetNodesByNames)
	if err != nil {
		return "", fmt.Errorf("error converting flow to scheduler model: %w", err)
	}

	// Create flow execution payload for scheduler
	payload := scheduler.FlowExecutionPayload{
		Workflow:          schedulerFlow,
		Input:             input,
		StartingActionIdx: actionIndex,
		ExecID:            execID,
		NamespaceID:       namespaceID,
		TriggerType:       scheduler.TriggerTypeManual,
		UserUUID:          userUUID,
	}

	// Create execution log for manual flows before queuing (needed for immediate API calls)
	inputB, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("could not marshal input to json: %w", err)
	}

	_, err = c.store.AddExecutionLog(ctx, repo.AddExecutionLogParams{
		ExecID:      execID,
		FlowID:      f.Meta.DBID,
		Input:       inputB,
		TriggerType: repo.TriggerTypeManual,
		Uuid:        userID,
		Uuid_2:      namespaceUUID,
	})
	if err != nil {
		return "", fmt.Errorf("could not add entry to execution log: %w", err)
	}

	// Queue the task using the scheduler
	_, err = c.scheduler.QueueTask(ctx, payload)
	if err != nil {
		return "", err
	}

	return execID, nil
}

// CancelFlowExecution cancels the given execution using the scheduler
func (c *Core) CancelFlowExecution(ctx context.Context, execID string) error {
	return c.scheduler.CancelTask(ctx, execID)
}

func (c *Core) GetExecutionSummaryPaginated(ctx context.Context, f models.Flow, namespaceID string, limit, offset int) ([]models.ExecutionSummary, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	execs, err := c.store.GetExecutionsByFlowPaginated(ctx, repo.GetExecutionsByFlowPaginatedParams{
		ID:     f.Meta.DBID,
		Uuid:   namespaceUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get paginated executions for %s: %w", f.Meta.ID, err)
	}

	var m []models.ExecutionSummary
	var pageCount, totalCount int64

	for _, v := range execs {
		m = append(m, models.ExecutionSummary{
			ExecID:          v.ExecID,
			FlowName:        v.FlowName,
			FlowID:          v.FlowSlug,
			CreatedAt:       v.CreatedAt,
			CompletedAt:     v.UpdatedAt,
			TriggerType:     string(v.TriggerType),
			Status:          models.ExecutionStatus(v.Status),
			TriggeredByName: v.TriggeredByName,
			TriggeredByID:   v.TriggeredByUuid.String(),
			CurrentActionID: v.CurrentActionID.String,
		})
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return m, pageCount, totalCount, nil
}

func (c *Core) GetAllExecutionSummaryPaginated(ctx context.Context, namespaceID string, filter string, limit, offset int) ([]models.ExecutionSummary, int64, int64, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	execs, err := c.store.SearchExecutionsPaginated(ctx, repo.SearchExecutionsPaginatedParams{
		Uuid:    namespaceUUID,
		Column2: filter,
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("could not get all paginated executions: %w", err)
	}

	var m []models.ExecutionSummary
	var pageCount, totalCount int64

	for _, v := range execs {
		m = append(m, models.ExecutionSummary{
			ExecID:          v.ExecID,
			FlowName:        v.FlowName,
			FlowID:          v.FlowSlug,
			CreatedAt:       v.CreatedAt,
			CompletedAt:     v.UpdatedAt,
			TriggerType:     string(v.TriggerType),
			Status:          models.ExecutionStatus(v.Status),
			TriggeredByName: v.TriggeredByName,
			TriggeredByID:   v.TriggeredByUuid.String(),
			CurrentActionID: v.CurrentActionID.String,
		})
		pageCount = v.PageCount
		totalCount = v.TotalCount
	}

	return m, pageCount, totalCount, nil
}

func (c *Core) GetExecutionSummaryByExecID(ctx context.Context, execID string, namespaceID string) (models.ExecutionSummary, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	e, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return models.ExecutionSummary{}, fmt.Errorf("could not get exec %s by exec id: %w", execID, err)
	}

	return models.ExecutionSummary{
		ExecID:          execID,
		Input:           e.Input,
		FlowName:        e.FlowName,
		FlowID:          e.FlowSlug,
		Status:          models.ExecutionStatus(e.Status),
		CreatedAt:       e.CreatedAt,
		CompletedAt:     e.UpdatedAt,
		TriggerType:     string(e.TriggerType),
		TriggeredByName: e.TriggeredByName,
		TriggeredByID:   e.TriggeredByUuid.String(),
		CurrentActionID: e.CurrentActionID.String,
	}, nil
}

func (c *Core) GetInputForExec(ctx context.Context, execID string, namespaceID string) (map[string]interface{}, error) {
	var input map[string]interface{}
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	in, err := c.store.GetInputForExecByUUID(ctx, repo.GetInputForExecByUUIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting input for %s: %w", execID, err)
	}

	if err := json.Unmarshal(in, &input); err != nil {
		return nil, fmt.Errorf("error unmarshaling input for %s: %w", execID, err)
	}

	return input, nil
}

func (c *Core) GetExecutionByExecID(ctx context.Context, execID string, namespaceID string) (models.Execution, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return models.Execution{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	e, err := c.store.GetExecutionByExecID(ctx, repo.GetExecutionByExecIDParams{
		ExecID: execID,
		Uuid:   namespaceUUID,
	})
	if err != nil {
		return models.Execution{}, fmt.Errorf("could not get execution for exec %s: %w", execID, err)
	}

	var input map[string]interface{}
	if err := json.Unmarshal(e.Input, &input); err != nil {
		return models.Execution{}, fmt.Errorf("error unmarshaling input for %s: %w", execID, err)
	}

	u, err := c.store.GetUserByID(ctx, e.TriggeredBy)
	if err != nil {
		return models.Execution{}, fmt.Errorf("could not get trigger person for %s: %w", execID, err)
	}

	return models.Execution{
		ExecID:      e.ExecID,
		Version:     int64(e.Version),
		Input:       input,
		ErrorMsg:    e.Error.String,
		TriggeredBy: u.Uuid.String(),
	}, nil
}

func (c *Core) CreateFlow(ctx context.Context, f models.Flow, namespaceID string) error {
	c.rwf.RLock()
	if _, exists := c.flows[f.Meta.ID]; exists {
		return fmt.Errorf("flow with id %s already exists", f.Meta.ID)
	}
	c.rwf.RUnlock()

	// Remove duplicate schedules
	f.Meta.Schedules = removeDuplicateStrings(f.Meta.Schedules)

	n, err := c.GetNamespaceByID(ctx, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get namespace details for %s: %w", namespaceID, err)
	}

	namespaceDirPath := filepath.Join(c.flowDirectory, n.Name)
	if err := os.MkdirAll(namespaceDirPath, 0755); err != nil {
		return fmt.Errorf("could not create namespace directory: %w", err)
	}

	flowDir := filepath.Join(namespaceDirPath, f.Meta.ID)
	if err := os.MkdirAll(flowDir, 0755); err != nil {
		return fmt.Errorf("could not create flow directory: %w", err)
	}

	yamlFilePath := filepath.Join(flowDir, fmt.Sprintf("%s.yaml", f.Meta.ID))
	if _, err := os.Stat(yamlFilePath); err == nil {
		return fmt.Errorf("flow with this ID already exists: %w", err)
	}

	yamlData, err := models.MarshalFlow(f, models.FlowFormatYAML)
	if err != nil {
		return fmt.Errorf("could not marshal flow to YAML: %w", err)
	}

	if err := os.WriteFile(yamlFilePath, yamlData, 0644); err != nil {
		return fmt.Errorf("could not write flow file: %w", err)
	}

	importedFlow, namespaceUUID, err := c.importFlowFromFile(yamlFilePath, n.Name)
	if err != nil {
		return fmt.Errorf("could not import flow after creation: %w", err)
	}

	c.rwf.Lock()
	defer c.rwf.Unlock()
	c.flows[fmt.Sprintf("%s:%s", importedFlow.Meta.ID, namespaceUUID)] = importedFlow
	return nil
}

func (c *Core) UpdateFlow(ctx context.Context, f models.Flow, namespaceID string) error {
	c.rwf.RLock()
	if _, exists := c.flows[fmt.Sprintf("%s:%s", f.Meta.ID, namespaceID)]; !exists {
		return fmt.Errorf("flow with id %s does not exist", f.Meta.ID)
	}
	c.rwf.RUnlock()

	// Remove duplicate schedules
	f.Meta.Schedules = removeDuplicateStrings(f.Meta.Schedules)

	n, err := c.GetNamespaceByID(ctx, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get namespace details for %s: %w", namespaceID, err)
	}

	// Get the existing flow from database to retrieve the file path
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	existingFlow, err := c.store.GetFlowBySlug(ctx, repo.GetFlowBySlugParams{
		Slug: f.Meta.ID,
		Uuid: namespaceUUID,
	})
	if err != nil {
		return fmt.Errorf("could not get existing flow: %w", err)
	}

	flowFilePath := existingFlow.FilePath
	if _, err := os.Stat(flowFilePath); err != nil {
		return fmt.Errorf("flow file does not exist at %s: %w", flowFilePath, err)
	}

	// Detect the format of the existing file and use the same format
	format := detectFlowFormat(flowFilePath)
	flowData, err := models.MarshalFlow(f, format)
	if err != nil {
		return fmt.Errorf("could not marshal flow to %s: %w", format, err)
	}

	if err := os.WriteFile(flowFilePath, flowData, 0644); err != nil {
		return fmt.Errorf("could not write flow file: %w", err)
	}

	importedFlow, namespaceUUIDStr, err := c.importFlowFromFile(flowFilePath, n.Name)
	if err != nil {
		return fmt.Errorf("could not import flow after creation: %w", err)
	}

	c.rwf.Lock()
	defer c.rwf.Unlock()
	c.flows[fmt.Sprintf("%s:%s", importedFlow.Meta.ID, namespaceUUIDStr)] = importedFlow
	return nil
}

func (c *Core) DeleteFlow(ctx context.Context, flowID, namespaceID string) error {
	c.rwf.RLock()
	if _, exists := c.flows[fmt.Sprintf("%s:%s", flowID, namespaceID)]; !exists {
		return fmt.Errorf("flow with id %s does not exist", flowID)
	}
	c.rwf.RUnlock()

	n, err := c.GetNamespaceByID(ctx, namespaceID)
	if err != nil {
		return fmt.Errorf("could not get namespace details for %s: %w", namespaceID, err)
	}

	namespaceDirPath := filepath.Join(c.flowDirectory, n.Name)
	flowDir := filepath.Join(namespaceDirPath, flowID)

	if err := os.RemoveAll(flowDir); err != nil {
		return fmt.Errorf("could not delete flow: %w", err)
	}

	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return fmt.Errorf("invalid namespace UUID: %w", err)
	}

	if err := c.store.DeleteFlow(ctx, repo.DeleteFlowParams{
		Slug: flowID,
		Uuid: namespaceUUID,
	}); err != nil {
		return fmt.Errorf("could not delete flow %s from DB: %w", flowID, err)
	}

	c.rwf.Lock()
	defer c.rwf.Unlock()
	delete(c.flows, fmt.Sprintf("%s:%s", flowID, namespaceID))
	return nil
}

func (c *Core) LoadFlows() error {
	m := make(map[string]models.Flow)

	// Read immediate subdirectories
	entries, err := os.ReadDir(c.flowDirectory)
	if err != nil {
		return fmt.Errorf("error reading flow directory: %w", err)
	}

	// Each subdirectory in the root flows directory should be a namespace
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		namespaceDir := filepath.Join(c.flowDirectory, entry.Name())
		namespaceFlows, err := c.processNamespaceFlows(namespaceDir)
		if err != nil {
			log.Printf("could process flows from namespace %s: %v", entry.Name(), err)
			continue
		}

		maps.Copy(m, namespaceFlows)
	}
	c.flows = m
	return nil
}

// processNamespaceFlows iterates through directories in the namespace directory and imports the first yaml file per directory as flow. The files are sorted by name.
func (c *Core) processNamespaceFlows(namespaceDir string) (map[string]models.Flow, error) {
	m := make(map[string]models.Flow)

	entries, err := os.ReadDir(namespaceDir)
	if err != nil {
		return nil, fmt.Errorf("error reading namespace %s directory: %w", namespaceDir, err)
	}

	namespaceName := filepath.Base(namespaceDir)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Find the flow file (YAML or HUML) in the flow directory
		flowDir := filepath.Join(namespaceDir, entry.Name())
		flowFiles, err := os.ReadDir(flowDir)
		if err != nil {
			log.Printf("error reading flow directory %s: %v", flowDir, err)
			continue
		}

		var flowPath string
		for _, file := range flowFiles {
			if !file.IsDir() && isFlowFile(file.Name()) {
				flowPath = filepath.Join(flowDir, file.Name())
				break
			}
		}

		if flowPath == "" {
			log.Printf("no flow file (YAML or HUML) found in flow directory %s", flowDir)
			continue
		}

		f, nsUUID, err := c.importFlowFromFile(flowPath, namespaceName)
		if err != nil {
			log.Printf("error importing flow from %s: %v", flowPath, err)
			continue
		}

		m[fmt.Sprintf("%s:%s", f.Meta.ID, nsUUID)] = f
	}

	return m, nil
}

func (c *Core) importFlowFromFile(flowFilePath, namespaceName string) (models.Flow, string, error) {
	data, err := os.ReadFile(flowFilePath)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("error reading file %s: %w", flowFilePath, err)
	}

	h := sha256.New()
	h.Write(data)
	checksum := hex.EncodeToString(h.Sum(nil))

	// Detect format based on file extension and unmarshal accordingly
	format := detectFlowFormat(flowFilePath)
	f, err := models.UnmarshalFlow(data, format)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("error parsing flow file in %s: %w", flowFilePath, err)
	}

	if err := f.Validate(); err != nil {
		return models.Flow{}, "", fmt.Errorf("validation error in %s: %w", flowFilePath, err)
	}

	f.Meta.SrcDir = filepath.Base(filepath.Dir(flowFilePath))
	if f.Meta.Namespace == "" {
		f.Meta.Namespace = namespaceName
	}

	if f.Meta.Namespace != namespaceName {
		return models.Flow{}, "", fmt.Errorf("flow namespace %s does not match expected namespace %s", f.Meta.Namespace, namespaceName)
	}

	ns, err := c.store.GetNamespaceByName(context.Background(), f.Meta.Namespace)
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("error getting namespace %s: %w", f.Meta.Namespace, err)
	}

	fd, err := c.store.GetFlowBySlug(context.Background(), repo.GetFlowBySlugParams{
		Slug: f.Meta.ID,
		Uuid: ns.Uuid,
	})
	if err != nil {
		fd, err = c.store.CreateFlow(context.Background(), repo.CreateFlowParams{
			Slug:          f.Meta.ID,
			Name:          f.Meta.Name,
			Checksum:      checksum,
			Description:   sql.NullString{String: f.Meta.Description, Valid: true},
			CronSchedules: f.Meta.Schedules,
			FilePath:      flowFilePath,
			Name_2:        f.Meta.Namespace,
		})
	} else if fd.Checksum != checksum {
		fd, err = c.store.UpdateFlow(context.Background(), repo.UpdateFlowParams{
			Name:          f.Meta.Name,
			Description:   sql.NullString{String: f.Meta.Description, Valid: true},
			Checksum:      checksum,
			CronSchedules: f.Meta.Schedules,
			FilePath:      flowFilePath,
			Slug:          f.Meta.ID,
			Name_2:        f.Meta.Namespace,
		})
	}
	if err != nil {
		return models.Flow{}, "", fmt.Errorf("database operation failed for flow %s: %w", f.Meta.ID, err)
	}

	f.Meta.DBID = fd.ID
	return f, ns.Uuid.String(), nil
}

// GetScheduledFlows returns all flows that have a cron schedule configured
func (c *Core) GetScheduledFlows() []models.Flow {
	c.rwf.RLock()
	defer c.rwf.RUnlock()

	ctx := context.Background()
	scheduledFlowRows, err := c.store.GetScheduledFlows(ctx)
	if err != nil {
		log.Printf("error getting scheduled flows from database: %v", err)
		return nil
	}

	var scheduledFlows []models.Flow
	for _, row := range scheduledFlowRows {
		flowKey := fmt.Sprintf("%s:%s", row.Slug, row.NamespaceUuid.String())
		if flow, exists := c.flows[flowKey]; exists {
			scheduledFlows = append(scheduledFlows, flow)
		}
	}

	return scheduledFlows
}

// GetSchedulerFlow loads a flow and converts it to scheduler.Flow format
// This function can be used as a FlowLoaderFn for the scheduler
func (c *Core) GetSchedulerFlow(ctx context.Context, flowSlug string, namespaceUUID string) (scheduler.Flow, error) {
	// Load the flow from the in-memory cache
	flow, err := c.GetFlowByID(flowSlug, namespaceUUID)
	if err != nil {
		return scheduler.Flow{}, fmt.Errorf("failed to get flow %s: %w", flowSlug, err)
	}

	// Parse namespace UUID
	nsUUID, err := uuid.Parse(namespaceUUID)
	if err != nil {
		return scheduler.Flow{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}

	// Convert to scheduler format with nodes resolved
	return models.ConvertToSchedulerFlow(ctx, flow, nsUUID, c.GetNodesByNames)
}

// removeDuplicateStrings removes duplicate strings from a slice
func removeDuplicateStrings(slice []string) []string {
	if len(slice) == 0 {
		return slice
	}

	seen := make(map[string]bool)
	result := make([]string, 0, len(slice))

	for _, item := range slice {
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}
