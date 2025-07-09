# Login
POST http://localhost:7000/login
Content-Type: application/json

{
  "username": "autopilot_admin",
  "password": "autopilot_password"
}

HTTP 200
[Asserts]
header "Set-Cookie" contains "session"

[Captures]
session: header "Set-Cookie" regex "session=([^;]+)"

# Test GET /api/v1/credentials
GET http://localhost:7000/api/v1/credentials
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.credentials" isCollection
jsonpath "$.page_count" isInteger
jsonpath "$.total_count" isInteger

# Test POST /api/v1/credentials
POST http://localhost:7000/api/v1/credentials
Content-Type: application/json
Cookie: session={{session}}

{
  "name": "TestCredential",
  "private_key": "test_private_key",
  "password": "test_password"
}

HTTP 201
[Asserts]
jsonpath "$.id" isString
jsonpath "$.name" == "TestCredential"

[Captures]
credId: jsonpath "$.id"

# Test GET /api/v1/credentials/:credID
GET http://localhost:7000/api/v1/credentials/{{credId}}
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{credId}}"
jsonpath "$.name" == "TestCredential"

# Test PUT /api/v1/credentials/:credID
PUT http://localhost:7000/api/v1/credentials/{{credId}}
Content-Type: application/json
Cookie: session={{session}}

{
  "name": "UpdatedTestCredential",
  "private_key": "updated_private_key",
  "password": "updated_password"
}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{credId}}"
jsonpath "$.name" == "UpdatedTestCredential"
jsonpath "$.private_key" == "updated_private_key"
jsonpath "$.password" == "updated_password"

# Test DELETE /api/v1/credentials/:credID
DELETE http://localhost:7000/api/v1/credentials/{{credId}}
Cookie: session={{session}}

HTTP 204

# Test GET /api/v1/groups
GET http://localhost:7000/api/v1/groups
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.groups" isCollection
jsonpath "$.page_count" isInteger
jsonpath "$.total_count" isInteger

# Test POST /api/v1/groups
POST http://localhost:7000/api/v1/groups
Content-Type: application/json
Cookie: session={{session}}

{
  "name": "TestGroup",
  "description": "A group created for testing"
}

HTTP 201
[Asserts]
jsonpath "$.id" isString
jsonpath "$.name" == "TestGroup"
jsonpath "$.description" == "A group created for testing"

[Captures]
groupId: jsonpath "$.id"

# Test GET /api/v1/groups/:groupID
GET http://localhost:7000/api/v1/groups/{{groupId}}
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{groupId}}"
jsonpath "$.name" == "TestGroup"

# Test PUT /api/v1/groups/:groupID
PUT http://localhost:7000/api/v1/groups/{{groupId}}
Content-Type: application/json
Cookie: session={{session}}

{
  "name": "UpdatedTestGroup",
  "description": "Updated description for testing"
}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{groupId}}"
jsonpath "$.name" == "UpdatedTestGroup"
jsonpath "$.description" == "Updated description for testing"

# Test DELETE /api/v1/groups/:groupID
DELETE http://localhost:7000/api/v1/groups/{{groupId}}
Cookie: session={{session}}

HTTP 204

# Test GET /api/v1/users
GET http://localhost:7000/api/v1/users
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.users" isCollection
jsonpath "$.page_count" isInteger
jsonpath "$.total_count" isInteger

# Test POST /api/v1/users
POST http://localhost:7000/api/v1/users
Content-Type: application/json
Cookie: session={{session}}

{
  "username": "testuser",
  "name": "Test User",
  "login_type": "local",
  "role": "user"
}

HTTP 201
[Asserts]
jsonpath "$.id" isString
jsonpath "$.username" == "testuser"
jsonpath "$.name" == "Test User"

[Captures]
userId: jsonpath "$.id"

# Test GET /api/v1/users/:userID
GET http://localhost:7000/api/v1/users/{{userId}}
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{userId}}"
jsonpath "$.username" == "testuser"

# Test PUT /api/v1/users/:userID
PUT http://localhost:7000/api/v1/users/{{userId}}
Content-Type: application/json
Cookie: session={{session}}

{
  "username": "updatedtestuser",
  "name": "Updated Test User",
  "login_type": "local",
  "role": "admin"
}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{userId}}"
jsonpath "$.username" == "updatedtestuser"
jsonpath "$.name" == "Updated Test User"
jsonpath "$.role" == "admin"

# Test DELETE /api/v1/users/:userID
DELETE http://localhost:7000/api/v1/users/{{userId}}
Cookie: session={{session}}

HTTP 204

# Test GET /api/v1/nodes
GET http://localhost:7000/api/v1/nodes
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.nodes" isCollection
jsonpath "$.page_count" isInteger
jsonpath "$.total_count" isInteger

# Test POST /api/v1/nodes
POST http://localhost:7000/api/v1/nodes
Content-Type: application/json
Cookie: session={{session}}

{
  "name": "TestNode",
  "hostname": "192.168.1.100",
  "port": 22,
  "username": "testuser",
  "os_family": "linux",
  "tags": ["dev", "test"],
  "auth": {
    "method": "ssh_key",
    "credential_id": "00000000-0000-0000-0000-000000000001"
  }
}

HTTP 201
[Asserts]
jsonpath "$.id" isString
jsonpath "$.name" == "TestNode"
jsonpath "$.hostname" == "192.168.1.100"

[Captures]
nodeId: jsonpath "$.id"

# Test GET /api/v1/nodes/:nodeID
GET http://localhost:7000/api/v1/nodes/{{nodeId}}
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{nodeId}}"
jsonpath "$.name" == "TestNode"

# Test PUT /api/v1/nodes/:nodeId
PUT http://localhost:7000/api/v1/nodes/{{nodeId}}
Content-Type: application/json
Cookie: session={{session}}

{
  "name": "UpdatedTestNode",
  "hostname": "192.168.1.101",
  "port": 22,
  "username": "updateduser",
  "os_family": "linux",
  "tags": ["prod"],
  "auth": {
    "method": "ssh_key",
    "credential_id": "00000000-0000-0000-0000-000000000001"
  }
}

HTTP 200
[Asserts]
jsonpath "$.id" == "{{nodeId}}"
jsonpath "$.name" == "UpdatedTestNode"
jsonpath "$.hostname" == "192.168.1.101"
jsonpath "$.username" == "updateduser"
jsonpath "$.tags[0]" == "prod"

# Test DELETE /api/v1/nodes/:nodeID
DELETE http://localhost:7000/api/v1/nodes/{{nodeId}}
Cookie: session={{session}}

HTTP 204

# Test POST /api/v1/trigger/:flow
POST http://localhost:7000/api/v1/trigger/example-flow
Content-Type: application/json
Cookie: session={{session}}

{}

HTTP 200
[Asserts]
jsonpath "$.flow_id" isString
jsonpath "$.log_id" isString

[Captures]
logId: jsonpath "$.log_id"

# Test GET /api/v1/logs/:logID
GET http://localhost:7000/api/v1/logs/{{logId}}
Cookie: session={{session}}

HTTP 200
[Asserts]
jsonpath "$" isCollection

# Test POST /api/v1/approvals/:approvalID
# This requires an existing approval, so using a placeholder ID
POST http://localhost:7000/api/v1/approvals/00000000-0000-0000-0000-000000000001
Content-Type: application/json
Cookie: session={{session}}

{
  "action": "approve"
}

HTTP 200
[Asserts]
jsonpath "$.id" isString
jsonpath "$.status" isString
