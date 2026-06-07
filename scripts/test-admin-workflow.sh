#!/bin/bash
# Test script for admin creation workflow
# This script validates the complete admin creation process

set -e  # Exit on error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8080}"
API_BASE="${BASE_URL}/api/v1"

SUPERADMIN_EMAIL="superadmin-test@magic.tn"
SUPERADMIN_PASSWORD="SuperAdmin123!"

ADMIN_EMAIL="admin-test@magic.tn"
ADMIN_PASSWORD="Admin123!"

USER_EMAIL="user-test@magic.tn"
USER_PASSWORD="User123!"

# Cleanup function
cleanup() {
    log_info "Cleaning up test data..."
    # TODO: Add cleanup logic if needed
}

trap cleanup EXIT

# Test 1: Create superadmin via CLI
test_create_superadmin_cli() {
    log_info "Test 1: Creating superadmin via CLI..."

    cd "$PROJECT_ROOT"

    if ! ADMIN_EMAIL="$SUPERADMIN_EMAIL" \
         ADMIN_PASSWORD="$SUPERADMIN_PASSWORD" \
         go run ./cmd/createadmin >/dev/null 2>&1; then
        log_error "Failed to create superadmin via CLI"
        return 1
    fi

    log_info "✓ Superadmin created successfully via CLI"
    return 0
}

# Test 2: Login as superadmin
test_login_superadmin() {
    log_info "Test 2: Logging in as superadmin..."

    response=$(curl -s -X POST "$API_BASE/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$SUPERADMIN_EMAIL\",\"password\":\"$SUPERADMIN_PASSWORD\"}")

    token=$(echo "$response" | jq -r '.token // .access_token // empty')

    if [ -z "$token" ] || [ "$token" = "null" ]; then
        log_error "Failed to login as superadmin"
        echo "Response: $response"
        return 1
    fi

    export SUPERADMIN_TOKEN="$token"
    log_info "✓ Superadmin login successful"
    return 0
}

# Test 3: Create admin via API (as superadmin)
test_create_admin_api() {
    log_info "Test 3: Creating admin via API (as superadmin)..."

    if [ -z "$SUPERADMIN_TOKEN" ]; then
        log_error "Superadmin token not available"
        return 1
    fi

    response=$(curl -s -X POST "$API_BASE/users/admins" \
        -H "Authorization: Bearer $SUPERADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\":\"$ADMIN_EMAIL\",
            \"password\":\"$ADMIN_PASSWORD\",
            \"username\":\"admin-test\",
            \"firstName\":\"Admin\",
            \"lastName\":\"Test\"
        }")

    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        log_error "Failed to create admin via API"
        echo "Response: $response"
        return 1
    fi

    log_info "✓ Admin created successfully via API"
    return 0
}

# Test 4: Login as admin
test_login_admin() {
    log_info "Test 4: Logging in as admin..."

    response=$(curl -s -X POST "$API_BASE/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")

    token=$(echo "$response" | jq -r '.token // .access_token // empty')

    if [ -z "$token" ] || [ "$token" = "null" ]; then
        log_error "Failed to login as admin"
        echo "Response: $response"
        return 1
    fi

    export ADMIN_TOKEN="$token"
    log_info "✓ Admin login successful"
    return 0
}

# Test 5: Admin tries to create another admin (should fail)
test_admin_cannot_create_admin() {
    log_info "Test 5: Testing that admin cannot create another admin..."

    if [ -z "$ADMIN_TOKEN" ]; then
        log_error "Admin token not available"
        return 1
    fi

    response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE/users/admins" \
        -H "Authorization: Bearer $ADMIN_TOKEN" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\":\"another-admin@magic.tn\",
            \"password\":\"Another123!\",
            \"username\":\"another-admin\"
        }")

    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" != "403" ]; then
        log_error "Admin should not be able to create another admin (expected 403, got $http_code)"
        return 1
    fi

    log_info "✓ Admin correctly denied from creating another admin"
    return 0
}

# Test 6: Create regular user
test_create_regular_user() {
    log_info "Test 6: Creating regular user..."

    response=$(curl -s -X POST "$API_BASE/users" \
        -H "Content-Type: application/json" \
        -d "{
            \"email\":\"$USER_EMAIL\",
            \"password\":\"$USER_PASSWORD\",
            \"username\":\"user-test\"
        }")

    if echo "$response" | jq -e '.error' >/dev/null 2>&1; then
        log_error "Failed to create regular user"
        echo "Response: $response"
        return 1
    fi

    log_info "✓ Regular user created successfully"
    return 0
}

# Test 7: Login as regular user
test_login_regular_user() {
    log_info "Test 7: Logging in as regular user..."

    response=$(curl -s -X POST "$API_BASE/auth/login" \
        -H "Content-Type: application/json" \
        -d "{\"email\":\"$USER_EMAIL\",\"password\":\"$USER_PASSWORD\"}")

    token=$(echo "$response" | jq -r '.token // .access_token // empty')

    if [ -z "$token" ] || [ "$token" = "null" ]; then
        log_error "Failed to login as regular user"
        echo "Response: $response"
        return 1
    fi

    export USER_TOKEN="$token"
    log_info "✓ Regular user login successful"
    return 0
}

# Test 8: Regular user tries to access admin route (should fail)
test_user_cannot_access_admin_route() {
    log_info "Test 8: Testing that regular user cannot access admin routes..."

    if [ -z "$USER_TOKEN" ]; then
        log_error "User token not available"
        return 1
    fi

    response=$(curl -s -w "\n%{http_code}" -X GET "$API_BASE/users" \
        -H "Authorization: Bearer $USER_TOKEN")

    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" != "403" ]; then
        log_error "Regular user should not access admin routes (expected 403, got $http_code)"
        return 1
    fi

    log_info "✓ Regular user correctly denied from admin routes"
    return 0
}

# Test 9: Admin can access admin routes
test_admin_can_access_admin_routes() {
    log_info "Test 9: Testing that admin can access admin routes..."

    if [ -z "$ADMIN_TOKEN" ]; then
        log_error "Admin token not available"
        return 1
    fi

    response=$(curl -s -w "\n%{http_code}" -X GET "$API_BASE/users" \
        -H "Authorization: Bearer $ADMIN_TOKEN")

    http_code=$(echo "$response" | tail -n1)

    if [ "$http_code" != "200" ]; then
        log_error "Admin should access admin routes (expected 200, got $http_code)"
        return 1
    fi

    log_info "✓ Admin can access admin routes"
    return 0
}

# Main test execution
main() {
    log_info "Starting admin creation workflow tests..."
    log_info "Base URL: $BASE_URL"
    echo ""

    # Check dependencies
    if ! command -v jq &> /dev/null; then
        log_error "jq is required but not installed. Install it with: brew install jq"
        exit 1
    fi

    if ! command -v curl &> /dev/null; then
        log_error "curl is required but not installed"
        exit 1
    fi

    # Run tests
    failed=0

    test_create_superadmin_cli || failed=$((failed + 1))
    test_login_superadmin || failed=$((failed + 1))
    test_create_admin_api || failed=$((failed + 1))
    test_login_admin || failed=$((failed + 1))
    test_admin_cannot_create_admin || failed=$((failed + 1))
    test_create_regular_user || failed=$((failed + 1))
    test_login_regular_user || failed=$((failed + 1))
    test_user_cannot_access_admin_route || failed=$((failed + 1))
    test_admin_can_access_admin_routes || failed=$((failed + 1))

    echo ""
    if [ $failed -eq 0 ]; then
        log_info "=========================================="
        log_info "All tests passed! ✓"
        log_info "=========================================="
        exit 0
    else
        log_error "=========================================="
        log_error "$failed test(s) failed! ✗"
        log_error "=========================================="
        exit 1
    fi
}

# Run main function
main "$@"

