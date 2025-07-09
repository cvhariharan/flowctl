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
