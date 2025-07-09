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
