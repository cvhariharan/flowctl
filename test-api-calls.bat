@echo off
REM CURL tests for remote options API

echo ========================================
echo Remote Options API Testing Script
echo ========================================
echo.

REM Check if mock API server is running
echo Checking if mock API server is running...
curl -s http://localhost:3000/health > nul 2>&1
if %errorlevel% neq 0 (
    echo.
    echo ERROR: Mock API server is not running!
    echo Please start it with: node test-api-server.js
    exit /b 1
)
echo ✓ Mock API server is running
echo.

echo ========================================
echo TEST 1: Fetch Environments (Basic)
echo ========================================
curl -s http://localhost:3000/api/environments | more
echo.
echo.

echo ========================================
echo TEST 2: Fetch Cities for United States
echo ========================================
curl -s "http://localhost:3000/api/cities?country=United%%20States" | more
echo.
echo.

echo ========================================
echo TEST 3: Fetch Cities for Canada
echo ========================================
curl -s "http://localhost:3000/api/cities?country=Canada" | more
echo.
echo.

echo ========================================
echo TEST 4: Fetch Regions for New York, United States
echo ========================================
curl -s "http://localhost:3000/api/regions?city=New%%20York&country=United%%20States" | more
echo.
echo.

echo ========================================
echo TEST 5: Fetch Regions for Toronto, Canada
echo ========================================
curl -s "http://localhost:3000/api/regions?city=Toronto&country=Canada" | more
echo.
echo.

echo ========================================
echo TEST 6: Fetch Services for production
echo ========================================
curl -s "http://localhost:3000/api/services?env=production" | more
echo.
echo.

echo ========================================
echo TEST 7: Fetch Services for development
echo ========================================
curl -s "http://localhost:3000/api/services?env=development" | more
echo.
echo.

echo ========================================
echo All API tests completed!
echo ========================================
echo.
pause
