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
