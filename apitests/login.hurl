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
