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
