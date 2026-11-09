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

const nodeCSVHeader = "name,hostname,port,username,connection_type,credential,tags\n"
const nodeCSVRecord = "web_01,192.0.2.1,22,admin,ssh,my_ssh_key,\"web, prod_1\"\n"

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
		{name: "normalized header", input: " NAME , HOSTNAME , PORT , USERNAME , CONNECTION_TYPE , CREDENTIAL , TAGS \n" + nodeCSVRecord, lines: []int{2}},
		{name: "short row", input: nodeCSVHeader + "short,localhost\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}, rowError: csv.ErrFieldCount},
		{name: "long row", input: nodeCSVHeader + "long,localhost,22,admin,ssh,my_ssh_key,,extra\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}, rowError: csv.ErrFieldCount},
		{name: "bad quote", input: nodeCSVHeader + "bad\"name,localhost,22,admin,ssh,my_ssh_key,\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}, rowError: csv.ErrBareQuote},
		{name: "unclosed quote", input: nodeCSVHeader + "\"bad,localhost,22,admin,ssh,my_ssh_key,\n", errorLines: []int{2}, rowError: csv.ErrQuote},
		{name: "multiline record", input: nodeCSVHeader + "\"web\n01\",localhost,22,admin,ssh,my_ssh_key,\n" + nodeCSVRecord, lines: []int{2, 4}},
		{name: "multiline parse error", input: nodeCSVHeader + "\"web\n01\",localhost\n" + nodeCSVRecord, lines: []int{4}, errorLines: []int{2}, rowError: csv.ErrFieldCount},
		{name: "invalid port", input: nodeCSVHeader + "bad,localhost,no,admin,ssh,my_ssh_key,\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}},
		{name: "empty", fatal: true},
		{name: "blank", input: "\n\n", fatal: true},
		{name: "header only", input: nodeCSVHeader, fatal: true},
		{name: "header and blank lines", input: nodeCSVHeader + "\n\n", fatal: true},
		{name: "missing header", input: nodeCSVRecord, fatal: true},
		{name: "short header", input: "name,hostname\n" + nodeCSVRecord, fatal: true},
		{name: "wrong header", input: strings.Replace(nodeCSVHeader, "hostname", "host", 1) + nodeCSVRecord, fatal: true},
		{name: "missing credential column", input: "name,hostname,port,username,connection_type,tags\n" + nodeCSVRecord, fatal: true},
		{name: "empty credential", input: nodeCSVHeader + "bad,localhost,22,admin,ssh,,\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}},
		{name: "blank credential", input: nodeCSVHeader + "bad,localhost,22,admin,ssh,   ,\n" + nodeCSVRecord, lines: []int{3}, errorLines: []int{2}},
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
				if rows[len(rows)-1].CredentialName != "my_ssh_key" {
					t.Fatalf("credential name = %q", rows[len(rows)-1].CredentialName)
				}
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
			if count, _, err := c.ImportNodes(context.Background(), r, uuid.NewString()); count != 0 || !errors.Is(err, readErr) {
				t.Fatalf("import count=%d, error=%v", count, err)
			}
		})
	}
}

type nodeImportStore struct {
	repo.Store
	credential       repo.GetCredentialByUUIDRow
	otherCredentials []repo.GetCredentialByUUIDRow
	nodes            map[string]repo.Node
	writeErr         error
	credentialErr    error
}

func (s *nodeImportStore) GetCredentialByUUID(_ context.Context, p repo.GetCredentialByUUIDParams) (repo.GetCredentialByUUIDRow, error) {
	if s.credentialErr != nil {
		return repo.GetCredentialByUUIDRow{}, s.credentialErr
	}
	for _, credential := range append([]repo.GetCredentialByUUIDRow{s.credential}, s.otherCredentials...) {
		if p.Uuid == credential.Uuid && p.Uuid_2 == credential.NamespaceUuid {
			return credential, nil
		}
	}
	return repo.GetCredentialByUUIDRow{}, sql.ErrNoRows
}
func (s *nodeImportStore) GetCredentialByName(_ context.Context, p repo.GetCredentialByNameParams) (repo.GetCredentialByNameRow, error) {
	if s.credentialErr != nil {
		return repo.GetCredentialByNameRow{}, s.credentialErr
	}
	for _, credential := range append([]repo.GetCredentialByUUIDRow{s.credential}, s.otherCredentials...) {
		if p.Name == credential.Name && p.Uuid == credential.NamespaceUuid {
			return repo.GetCredentialByNameRow(credential), nil
		}
	}
	return repo.GetCredentialByNameRow{}, sql.ErrNoRows
}
func (s *nodeImportStore) CreateNode(_ context.Context, p repo.CreateNodeParams) (repo.Node, error) {
	if s.writeErr != nil {
		return repo.Node{}, s.writeErr
	}
	if _, exists := s.nodes[p.Name]; exists {
		return repo.Node{}, fmt.Errorf("insert node: %w", &pq.Error{Code: "23505", Constraint: "nodes_name_namespace_id_key"})
	}
	n := repo.Node{Uuid: uuid.New(), Name: p.Name, Hostname: p.Hostname, Port: p.Port, Username: p.Username, ConnectionType: p.ConnectionType, AuthMethod: p.AuthMethod, CredentialID: p.CredentialID, Tags: p.Tags}
	s.nodes[p.Name] = n
	return n, nil
}
func (s *nodeImportStore) UpdateNode(_ context.Context, p repo.UpdateNodeParams) (repo.Node, error) {
	return repo.Node{}, s.writeErr
}
func newNodeImportCore() (*Core, *nodeImportStore) {
	s := &nodeImportStore{credential: repo.GetCredentialByUUIDRow{ID: 1, Uuid: uuid.New(), Name: "my_ssh_key", KeyType: "private_key", NamespaceUuid: uuid.New()}, nodes: make(map[string]repo.Node)}
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
		"existing,localhost,22,admin,ssh,my_ssh_key,\n" +
		"bad_host,foo bar;baz,22,admin,ssh,my_ssh_key,\n" +
		"bad_tag,localhost,22,admin,ssh,my_ssh_key,web-prod\n" +
		"local,localhost,22,admin,ssh,my_ssh_key,\n" +
		"bad_port,localhost,65536,admin,ssh,my_ssh_key,\n" +
		"valid,localhost,22,admin,qssh,my_ssh_key,\n"
	count, errs, err := c.ImportNodes(context.Background(), strings.NewReader(input), s.credential.NamespaceUuid.String())
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
		if count, _, err := c.ImportNodes(context.Background(), strings.NewReader(input), uuid.NewString()); count != 0 || !errors.Is(err, ErrInvalidNodeCSV) {
			t.Fatalf("import count=%d, error=%v", count, err)
		}
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
					_, err = c.CreateNode(context.Background(), &n, s.credential.NamespaceUuid.String())
				} else {
					_, err = c.UpdateNode(context.Background(), uuid.NewString(), &n, s.credential.NamespaceUuid.String())
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

func TestImportNodesCredentialsByName(t *testing.T) {
	c, s := newNodeImportCore()
	namespace := s.credential.NamespaceUuid
	password := repo.GetCredentialByUUIDRow{ID: 2, Uuid: uuid.New(), Name: "Prod, Password", KeyType: "password", NamespaceUuid: namespace}
	foreign := repo.GetCredentialByUUIDRow{ID: 3, Uuid: uuid.New(), Name: "foreign_key", KeyType: "private_key", NamespaceUuid: uuid.New()}
	// A name used in another namespace must resolve only to the local credential.
	foreignSameName := repo.GetCredentialByUUIDRow{ID: 4, Uuid: uuid.New(), Name: password.Name, KeyType: "private_key", NamespaceUuid: foreign.NamespaceUuid}
	s.otherCredentials = []repo.GetCredentialByUUIDRow{foreignSameName, password, foreign}
	input := nodeCSVHeader + nodeCSVRecord +
		"password_node,localhost,22,admin,ssh,\"  Prod, Password  \",\n" +
		"unknown,localhost,22,admin,ssh,missing_key,\n" +
		"foreign,localhost,22,admin,ssh,foreign_key,\n" +
		"wrong_case,localhost,22,admin,ssh,MY_SSH_KEY,\n" +
		"missing,localhost,22,admin,ssh,,\n" +
		"valid,localhost,22,admin,qssh,my_ssh_key,\n"
	count, errs, err := c.ImportNodes(context.Background(), strings.NewReader(input), namespace.String())
	if err != nil || count != 3 || len(errs) != 4 {
		t.Fatalf("imported=%d, row errors=%v, error=%v", count, errs, err)
	}
	byName := make(map[string]nodeCSVError)
	for _, rowErr := range errs {
		byName[rowErr.Name] = rowErr
	}
	for name, line := range map[string]int{"unknown": 4, "foreign": 5, "wrong_case": 6, "missing": 7} {
		rowErr, ok := byName[name]
		if !ok || rowErr.Line != line {
			t.Fatalf("row error %q = %+v, want line %d", name, rowErr, line)
		}
		if name == "missing" {
			if rowErr.Err.Error() != "credential is required" {
				t.Fatalf("missing credential error = %v", rowErr.Err)
			}
		} else if !errors.Is(rowErr.Err, ErrNodeCredentialNotFound) {
			t.Fatalf("credential lookup error = %v", rowErr.Err)
		}
	}
	for name, credential := range map[string]repo.GetCredentialByUUIDRow{"web_01": s.credential, "password_node": password, "valid": s.credential} {
		n := s.nodes[name]
		if n.CredentialID.Int32 != credential.ID || !n.CredentialID.Valid || string(n.AuthMethod) != credential.KeyType {
			t.Fatalf("node %q uses credential %+v with method %s, want %d/%s", name, n.CredentialID, n.AuthMethod, credential.ID, credential.KeyType)
		}
	}
}

func TestImportNodesCredentialLookupFailure(t *testing.T) {
	c, s := newNodeImportCore()
	s.credentialErr = errors.New("database connection failed: sensitive details")
	count, errs, err := c.ImportNodes(context.Background(), strings.NewReader(nodeCSVHeader+nodeCSVRecord), s.credential.NamespaceUuid.String())
	if err != nil || count != 0 || len(errs) != 1 || errs[0].Line != 2 || errs[0].Err.Error() != `could not look up credential "my_ssh_key"` {
		t.Fatalf("imported=%d, row errors=%v, error=%v", count, errs, err)
	}
}
