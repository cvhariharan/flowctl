package core

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/cvhariharan/flowctl/internal/core/models"
	"github.com/cvhariharan/flowctl/internal/repo"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

const nodeCSVHeader = "name,hostname,port,username,connection_type,tags\n"
const nodeCSVRecord = "web_01,192.0.2.1,22,admin,ssh,\"web, prod_1\"\n"

func TestParseNodesCSV(t *testing.T) {
	for _, tt := range []struct {
		name, input       string
		lines, errorLines []int
		rowError          error
		fatal             bool
	}{
		{name: "valid", input: nodeCSVHeader + nodeCSVRecord, lines: []int{2}},
		{name: "BOM", input: "\ufeff" + nodeCSVHeader + nodeCSVRecord, lines: []int{2}},
		{name: "CRLF", input: strings.ReplaceAll(nodeCSVHeader+nodeCSVRecord, "\n", "\r\n"), lines: []int{2}},
		{name: "blank lines", input: "\n" + nodeCSVHeader + "\n" + nodeCSVRecord, lines: []int{4}},
		{name: "normalized header", input: " NAME , HOSTNAME , PORT , USERNAME , CONNECTION_TYPE , TAGS \n" + nodeCSVRecord, lines: []int{2}},
		{name: "short row", input: nodeCSVHeader + "short,localhost\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}, rowError: csv.ErrFieldCount},
		{name: "long row", input: nodeCSVHeader + "long,localhost,22,admin,ssh,,extra\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}, rowError: csv.ErrFieldCount},
		{name: "bad quote", input: nodeCSVHeader + "bad\"name,localhost,22,admin,ssh,\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}, rowError: csv.ErrBareQuote},
		{name: "unclosed quote", input: nodeCSVHeader + "\"bad,localhost,22,admin,ssh,\n", errorLines: []int{2}, rowError: csv.ErrQuote},
		{name: "multiline record", input: nodeCSVHeader + "\"web\n01\",localhost,22,admin,ssh,\n" + nodeCSVRecord, lines: []int{2, 4}},
		{name: "multiline parse error", input: nodeCSVHeader + "\"web\n01\",localhost\n" + nodeCSVRecord, lines: []int{4}, errorLines: []int{2}, rowError: csv.ErrFieldCount},
		{name: "invalid port", input: nodeCSVHeader + "bad,localhost,no,admin,ssh,\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}},
		{name: "empty", fatal: true},
		{name: "blank", input: "\n\n", fatal: true},
		{name: "header only", input: nodeCSVHeader, fatal: true},
		{name: "header and blank lines", input: nodeCSVHeader + "\n\n", fatal: true},
		{name: "missing header", input: nodeCSVRecord, fatal: true},
		{name: "short header", input: "name,hostname\n" + nodeCSVRecord, fatal: true},
		{name: "wrong header", input: strings.Replace(nodeCSVHeader, "hostname", "host", 1) + nodeCSVRecord, fatal: true},
		{name: "bad header quoting", input: "na\"me,hostname,port,username,connection_type,tags\n", fatal: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, errs, err := parseNodesCSV(strings.NewReader(tt.input))
			if tt.fatal {
				if !errors.Is(err, ErrInvalidNodeCSV) {
					t.Fatalf("error = %v, want ErrInvalidNodeCSV", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var lines, errorLines []int
			for _, row := range rows {
				lines = append(lines, row.Line)
			}
			for _, rowErr := range errs {
				errorLines = append(errorLines, rowErr.Line)
				if tt.rowError != nil && !errors.Is(rowErr.Err, tt.rowError) {
					t.Fatalf("row error = %v, want %v", rowErr.Err, tt.rowError)
				}
			}
			if !reflect.DeepEqual(lines, tt.lines) || !reflect.DeepEqual(errorLines, tt.errorLines) {
				t.Fatalf("lines = %v, errors = %v; want %v, %v", lines, errorLines, tt.lines, tt.errorLines)
			}
			if len(rows) > 0 {
				last := rows[len(rows)-1].Node
				if last.Name != "web_01" || last.Port != 22 || !reflect.DeepEqual(last.Tags, []string{"web", "prod_1"}) {
					t.Fatalf("unexpected parsed node: %+v", last)
				}
			}
		})
	}
}

func TestNodeCSVFatalReadError(t *testing.T) {
	readErr := errors.New("disk read failed")
	for _, prefix := range []string{"", nodeCSVHeader, nodeCSVHeader + nodeCSVRecord} {
		t.Run(fmt.Sprintf("after %d bytes", len(prefix)), func(t *testing.T) {
			r := io.MultiReader(strings.NewReader(prefix), iotest.ErrReader(readErr))
			rows, errs, err := parseNodesCSV(r)
			if !errors.Is(err, readErr) || rows != nil || errs != nil {
				t.Fatalf("got rows=%v, errors=%v, error=%v", rows, errs, err)
			}
			c := &Core{} // Fatal reads must return before any database access.
			r = io.MultiReader(strings.NewReader(prefix), iotest.ErrReader(readErr))
			if count, _, err := c.ImportNodes(context.Background(), r, uuid.NewString(), uuid.NewString()); count != 0 || !errors.Is(err, readErr) {
				t.Fatalf("import count=%d, error=%v", count, err)
			}
		})
	}
}

type nodeImportStore struct {
	repo.Store
	credential    repo.GetCredentialByUUIDRow
	nodes         map[string]repo.Node
	writeErr      error
	credentialErr error
}

func (s *nodeImportStore) GetCredentialByUUID(_ context.Context, p repo.GetCredentialByUUIDParams) (repo.GetCredentialByUUIDRow, error) {
	if s.credentialErr != nil {
		return repo.GetCredentialByUUIDRow{}, s.credentialErr
	}
	if p.Uuid != s.credential.Uuid {
		return repo.GetCredentialByUUIDRow{}, sql.ErrNoRows
	}
	return s.credential, nil
}
func (s *nodeImportStore) CreateNode(_ context.Context, p repo.CreateNodeParams) (repo.Node, error) {
	if s.writeErr != nil {
		return repo.Node{}, s.writeErr
	}
	if _, exists := s.nodes[p.Name]; exists {
		return repo.Node{}, fmt.Errorf("insert node: %w", &pq.Error{Code: "23505", Constraint: "nodes_name_namespace_id_key"})
	}
	n := repo.Node{Uuid: uuid.New(), Name: p.Name, Hostname: p.Hostname, Port: p.Port, Username: p.Username, ConnectionType: p.ConnectionType, AuthMethod: p.AuthMethod, Tags: p.Tags}
	s.nodes[p.Name] = n
	return n, nil
}
func (s *nodeImportStore) UpdateNode(_ context.Context, p repo.UpdateNodeParams) (repo.Node, error) {
	return repo.Node{}, s.writeErr
}
func newNodeImportCore() (*Core, *nodeImportStore) {
	s := &nodeImportStore{credential: repo.GetCredentialByUUIDRow{ID: 1, Uuid: uuid.New(), KeyType: "private_key"}, nodes: make(map[string]repo.Node)}
	return &Core{store: s}, s
}
func validTestNode() models.Node {
	return models.Node{Name: "web_01", Hostname: "192.0.2.1", Port: 22, Username: "admin", ConnectionType: "ssh", Auth: models.NodeAuth{CredentialID: uuid.NewString(), Method: models.AuthMethodPrivateKey}}
}

func TestNodeMutationsValidate(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, name := range []string{"local", "uploads", "global", "bad-name", "bad_hostname"} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				n := validTestNode()
				n.Name = name
				if name == "bad_hostname" {
					n.Hostname = "foo bar;baz"
				}
				c := &Core{} // Invalid nodes must not reach the database.
				var err error
				if operation == "create" {
					_, err = c.CreateNode(context.Background(), &n, uuid.NewString())
				} else {
					_, err = c.UpdateNode(context.Background(), uuid.NewString(), &n, uuid.NewString())
				}
				if name == "bad-name" || name == "bad_hostname" {
					var validationErrors validator.ValidationErrors
					if !errors.As(err, &validationErrors) {
						t.Fatalf("error = %v, want validation error", err)
					}
				} else if !errors.Is(err, ErrNodeNameReserved) {
					t.Fatalf("error = %v, want ErrNodeNameReserved", err)
				}
			})
		}
	}
}

func TestImportNodes(t *testing.T) {
	c, s := newNodeImportCore()
	s.credential.KeyType = "password"
	s.nodes["existing"] = repo.Node{Name: "existing"}
	input := nodeCSVHeader + nodeCSVRecord + nodeCSVRecord +
		"existing,localhost,22,admin,ssh,\n" +
		"bad_host,foo bar;baz,22,admin,ssh,\n" +
		"bad_tag,localhost,22,admin,ssh,web-prod\n" +
		"local,localhost,22,admin,ssh,\n" +
		"bad_port,localhost,65536,admin,ssh,\n" +
		"valid,localhost,22,admin,qssh,\n"
	count, errs, err := c.ImportNodes(context.Background(), strings.NewReader(input), s.credential.Uuid.String(), uuid.NewString())
	if err != nil || count != 2 || len(errs) != 6 {
		t.Fatalf("imported=%d, row errors=%v, error=%v", count, errs, err)
	}
	for i, e := range errs {
		if e.Line != i+3 {
			t.Fatalf("row error %d line=%d, want %d", i, e.Line, i+3)
		}
		if i < 2 && (!errors.Is(e.Err, ErrNodeAlreadyExists) || e.Err.Error() != fmt.Sprintf("node %q already exists", e.Name)) {
			t.Fatalf("duplicate error = %v", e.Err)
		}
	}
	if s.nodes["valid"].AuthMethod != repo.AuthenticationMethodPassword {
		t.Fatalf("credential auth method not used: %+v", s.nodes["valid"])
	}
}

func TestImportNodesFileErrors(t *testing.T) {
	for _, input := range []string{"", nodeCSVHeader, "wrong,header\n"} {
		c := &Core{}
		if count, _, err := c.ImportNodes(context.Background(), strings.NewReader(input), uuid.NewString(), uuid.NewString()); count != 0 || !errors.Is(err, ErrInvalidNodeCSV) {
			t.Fatalf("import count=%d, error=%v", count, err)
		}
	}
	c, s := newNodeImportCore()
	s.credentialErr = sql.ErrNoRows
	if _, _, err := c.ImportNodes(context.Background(), strings.NewReader(nodeCSVHeader+nodeCSVRecord), s.credential.Uuid.String(), uuid.NewString()); !errors.Is(err, ErrNodeCredentialNotFound) {
		t.Fatalf("error = %v, want missing credential", err)
	}
}

func TestNodeDatabaseErrors(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, code := range []pq.ErrorCode{"23505", "23503"} {
			t.Run(operation+"/"+string(code), func(t *testing.T) {
				c, s := newNodeImportCore()
				s.writeErr = fmt.Errorf("query failed: %w", &pq.Error{Code: code})
				n := validTestNode()
				n.Auth.CredentialID = s.credential.Uuid.String()
				var err error
				if operation == "create" {
					_, err = c.CreateNode(context.Background(), &n, uuid.NewString())
				} else {
					_, err = c.UpdateNode(context.Background(), uuid.NewString(), &n, uuid.NewString())
				}
				if code == "23505" {
					if !errors.Is(err, ErrNodeAlreadyExists) {
						t.Fatalf("error = %v, want duplicate error", err)
					}
				} else if !errors.Is(err, s.writeErr) {
					t.Fatalf("database error lost: %v", err)
				}
			})
		}
	}
}
