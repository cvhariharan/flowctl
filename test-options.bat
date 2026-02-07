@echo off
REM Test script for remote options functionality (Windows)

echo Running remote options tests...
echo.

REM Run Go unit tests for the options package
echo 1. Running Go unit tests for options utility...
go test -v ./internal/utils -run TestInterpolateVariables -timeout 10s
if %errorlevel% neq 0 (
    echo ✗ Variable interpolation tests failed
    exit /b 1
)
echo ✓ Variable interpolation tests passed
echo.

go test -v ./internal/utils -run TestFetchRemoteOptions -timeout 10s
if %errorlevel% neq 0 (
    echo ✗ Remote options fetching tests failed
    exit /b 1
)
echo ✓ Remote options fetching tests passed
echo.

go test -v ./internal/utils -run TestFetchRemoteOptionsWithVariables -timeout 10s
if %errorlevel% neq 0 (
    echo ✗ Variable interpolation with remote fetch tests failed
    exit /b 1
)
echo ✓ Variable interpolation with remote fetch tests passed
echo.

REM Run all options tests
echo 2. Running all options tests...
go test -v ./internal/utils -timeout 30s
if %errorlevel% neq 0 (
    echo ✗ Some tests failed
    exit /b 1
)
echo ✓ All options tests passed
echo.

echo ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
echo All tests passed successfully!
echo ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
echo.
echo To test the integration with a live flow:
echo 1. Start the flowctl server
echo 2. Create a flow with select inputs using options_url
echo 3. Set up a mock API server using 'node test-api-server.js'
echo 4. Trigger the flow and observe the remote options being fetched
echo.
pause
