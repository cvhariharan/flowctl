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
