package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/google/uuid"
	"github.com/sergi/go-diff/diffmatchpatch"
)

var (
	ErrFlowVersionNotFound       = errors.New("flow version not found")
	ErrFlowVersionInvalid        = errors.New("flow version is invalid")
	ErrFlowVersionFormatMismatch = errors.New("flow version format does not match the flow file")
)

type FlowVersion struct {
	Version   int
	Checksum  string
	Format    string
	Source    string
	CreatedAt time.Time
	Active    bool
}

type DiffLine struct {
	Op      string `json:"op"`
	OldLine int    `json:"old,omitempty"`
	NewLine int    `json:"new,omitempty"`
	Text    string `json:"text"`
}

type FlowVersionDiff struct {
	From  int        `json:"from"`
	To    int        `json:"to"`
	Lines []DiffLine `json:"lines"`
}

func nullVersionDBID(id int32) sql.NullInt32 {
	return sql.NullInt32{Int32: id, Valid: id != 0}
}

func parseFlowSource(data []byte, format models.FlowFormat, namespaceName, srcDir string) (models.Flow, error) {
	f, err := models.UnmarshalFlow(data, format)
	if err != nil {
		return models.Flow{}, err
	}
	if err := f.Validate(); err != nil {
		return models.Flow{}, err
	}
	f.Meta.SrcDir = srcDir
	if f.Meta.Namespace == "" {
		f.Meta.Namespace = namespaceName
	}
	if f.Meta.Namespace != namespaceName {
		return models.Flow{}, fmt.Errorf("flow namespace %s does not match expected namespace %s", f.Meta.Namespace, namespaceName)
	}
	return f, nil
}

func (c *Core) writeAndImportFlowFile(ctx context.Context, flowFilePath, namespaceName string, data []byte) (models.Flow, error) {
	isSub, err := isSubpath(c.flowDirectory, flowFilePath)
	if err != nil || !isSub {
		return models.Flow{}, fmt.Errorf("cannot write to file outside flows root: %s", flowFilePath)
	}
	if err := os.WriteFile(flowFilePath, data, 0644); err != nil {
		return models.Flow{}, fmt.Errorf("could not write flow file: %w", err)
	}
	f, nsUUID, err := c.importFlowFromFile(ctx, flowFilePath, namespaceName)
	if err != nil {
		return models.Flow{}, fmt.Errorf("could not import flow: %w", err)
	}
	c.rwf.Lock()
	c.flows[fmt.Sprintf("%s:%s", f.Meta.ID, nsUUID)] = f
	c.rwf.Unlock()
	return f, nil
}

func (c *Core) flowVersionRow(ctx context.Context, flowSlug, namespaceID string, version int) (repo.GetFlowVersionRow, error) {
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return repo.GetFlowVersionRow{}, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	if version < 1 || int64(version) > 1<<31-1 {
		return repo.GetFlowVersionRow{}, ErrFlowVersionNotFound
	}
	row, err := c.store.GetFlowVersion(ctx, repo.GetFlowVersionParams{Slug: flowSlug, Uuid: namespaceUUID, Version: int32(version)})
	if errors.Is(err, sql.ErrNoRows) {
		return repo.GetFlowVersionRow{}, ErrFlowVersionNotFound
	}
	return row, err
}

func (c *Core) GetFlowVersion(ctx context.Context, flowSlug, namespaceID string, version int) (models.Flow, error) {
	current, err := c.GetFlowByID(flowSlug, namespaceID)
	if err != nil {
		return models.Flow{}, err
	}
	if version == 0 || current.Meta.Version == version {
		return current, nil
	}
	row, err := c.flowVersionRow(ctx, flowSlug, namespaceID, version)
	if err != nil {
		return models.Flow{}, err
	}
	f, err := parseFlowSource([]byte(row.Source), models.FlowFormat(row.Format), row.NamespaceName, filepath.Base(filepath.Dir(row.FilePath)))
	if err != nil {
		return models.Flow{}, fmt.Errorf("flow %s version %d can no longer be loaded: %w", flowSlug, version, err)
	}
	f.Meta.DBID, f.Meta.VersionDBID, f.Meta.Version = row.FlowID, row.ID, int(row.Version)
	return f, nil
}

func (c *Core) GetFlowVersionSource(ctx context.Context, flowSlug, namespaceID string, version int) (FlowVersion, error) {
	current, err := c.GetFlowByID(flowSlug, namespaceID)
	if err != nil {
		return FlowVersion{}, err
	}
	row, err := c.flowVersionRow(ctx, flowSlug, namespaceID, version)
	if err != nil {
		return FlowVersion{}, err
	}
	return FlowVersion{Version: int(row.Version), Checksum: row.Checksum, Format: row.Format, Source: row.Source, CreatedAt: row.CreatedAt, Active: current.Meta.Version == int(row.Version)}, nil
}

func (c *Core) ListFlowVersions(ctx context.Context, flowSlug, namespaceID string, limit, offset int) ([]FlowVersion, int64, int64, error) {
	current, err := c.GetFlowByID(flowSlug, namespaceID)
	if err != nil {
		return nil, 0, 0, err
	}
	namespaceUUID, err := uuid.Parse(namespaceID)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("invalid namespace UUID: %w", err)
	}
	rows, err := c.store.ListFlowVersions(ctx, repo.ListFlowVersionsParams{Slug: flowSlug, Uuid: namespaceUUID, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, 0, 0, err
	}
	versions := make([]FlowVersion, 0, len(rows))
	var pageCount, totalCount int64
	for _, row := range rows {
		versions = append(versions, FlowVersion{Version: int(row.Version), Checksum: row.Checksum, CreatedAt: row.CreatedAt, Active: int(row.Version) == current.Meta.Version})
		pageCount, totalCount = row.PageCount, row.TotalCount
	}
	return versions, pageCount, totalCount, nil
}

func (c *Core) ActivateFlowVersion(ctx context.Context, flowSlug, namespaceID string, version int) (models.Flow, error) {
	current, err := c.GetFlowByID(flowSlug, namespaceID)
	if err != nil {
		return models.Flow{}, err
	}
	if current.Meta.Version == version {
		return current, nil
	}
	row, err := c.flowVersionRow(ctx, flowSlug, namespaceID, version)
	if err != nil {
		return models.Flow{}, err
	}
	if _, err := parseFlowSource([]byte(row.Source), models.FlowFormat(row.Format), row.NamespaceName, filepath.Base(filepath.Dir(row.FilePath))); err != nil {
		return models.Flow{}, fmt.Errorf("%w: %v", ErrFlowVersionInvalid, err)
	}
	if models.FlowFormat(row.Format) != DetectFlowFormat(row.FilePath) {
		return models.Flow{}, ErrFlowVersionFormatMismatch
	}
	f, err := c.writeAndImportFlowFile(ctx, row.FilePath, row.NamespaceName, []byte(row.Source))
	if err != nil {
		return models.Flow{}, err
	}
	if f.Meta.Version != version {
		return models.Flow{}, fmt.Errorf("expected flow version %d after switching, got %d", version, f.Meta.Version)
	}
	return f, nil
}

func (c *Core) DiffFlowVersions(ctx context.Context, flowSlug, namespaceID string, from, to int) (FlowVersionDiff, error) {
	current, err := c.GetFlowByID(flowSlug, namespaceID)
	if err != nil {
		return FlowVersionDiff{}, err
	}
	if to == 0 {
		to = current.Meta.Version
	}
	a, err := c.flowVersionRow(ctx, flowSlug, namespaceID, from)
	if err != nil {
		return FlowVersionDiff{}, err
	}
	b, err := c.flowVersionRow(ctx, flowSlug, namespaceID, to)
	if err != nil {
		return FlowVersionDiff{}, err
	}
	return FlowVersionDiff{From: from, To: to, Lines: diffLines(a.Source, b.Source)}, nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func diffLines(a, b string) []DiffLine {
	dmp := diffmatchpatch.New()
	ra, rb, lines := dmp.DiffLinesToRunes(a, b)
	diffs := dmp.DiffCharsToLines(dmp.DiffMainRunes(ra, rb, false), lines)
	out := make([]DiffLine, 0)
	oldN, newN := 0, 0
	for _, d := range diffs {
		for _, text := range splitLines(d.Text) {
			switch d.Type {
			case diffmatchpatch.DiffEqual:
				oldN++
				newN++
				out = append(out, DiffLine{Op: "equal", OldLine: oldN, NewLine: newN, Text: text})
			case diffmatchpatch.DiffDelete:
				oldN++
				out = append(out, DiffLine{Op: "delete", OldLine: oldN, Text: text})
			case diffmatchpatch.DiffInsert:
				newN++
				out = append(out, DiffLine{Op: "insert", NewLine: newN, Text: text})
			}
		}
	}
	return out
}
