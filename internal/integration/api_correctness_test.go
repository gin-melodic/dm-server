package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"dm-server/internal/router"
	"dm-server/internal/utility/limiter"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gcfg"
)

var (
	testServer  *ghttp.Server
	mockService *MockService
	testConfig  TestConfig
)

func TestMain(m *testing.M) {
	// Set GoFrame configuration directory relative to this test file
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		_ = adapter.SetPath("../../manifest/config")
	}

	testConfig = LoadConfig()
	mockService = NewMockService(testConfig)
	mockService.Register()

	// Initialize Server and Rate Limiter
	ctx := context.Background()
	_ = limiter.Init(ctx)

	testServer = g.Server("test-server")
	serverRouter := router.NewServerRouter("/api")

	testServer.Group("/", func(group *ghttp.RouterGroup) {
		serverRouter.Register()(group)
	})

	testServer.SetPort(8000)
	testServer.SetDumpRouterMap(false)
	testServer.SetErrorLogEnabled(true)
	testServer.SetAccessLogEnabled(true)

	go testServer.Run()

	// Wait briefly for the server to bind and start
	time.Sleep(500 * time.Millisecond)

	code := m.Run()

	// Shutdown the server gracefully
	_ = testServer.Shutdown()
	os.Exit(code)
}

// Helpers for HTTP Requests in tests
type GoFrameResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func sendRequest(method, path string, body interface{}, token string) (int, *GoFrameResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(jsonBytes)
	}

	url := fmt.Sprintf("%s%s", testConfig.BaseURL, path)
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	// Try to parse GoFrame generic response
	var gfResp GoFrameResponse
	if err := json.Unmarshal(respBody, &gfResp); err != nil {
		// Not a standard GoFrame response or failed, return raw response data as message
		return resp.StatusCode, &GoFrameResponse{
			Code:    resp.StatusCode,
			Message: string(respBody),
		}, nil
	}

	return resp.StatusCode, &gfResp, nil
}

// TestWechatAuth tests /api/v1/wechat/auth
func TestWechatAuth(t *testing.T) {
	// Correct scenario
	body := g.Map{"code": "auth_code_123"}
	status, resp, err := sendRequest(http.MethodPost, "/v1/wechat/auth", body, "")
	if err != nil {
		t.Fatalf("WechatAuth request failed: %v", err)
	}
	if status != http.StatusOK || resp.Code != 0 {
		t.Errorf("Expected 200, got status: %d, code: %d, msg: %s", status, resp.Code, resp.Message)
	}
	var getData map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &getData); err != nil {
		t.Fatalf("Failed to parse user info response data: %v", err)
	}
	if getData["user_info"] == nil {
		t.Fatalf("Expected user_info in response")
	}

	// Check token presence in response data
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("Failed to parse response data: %v", err)
	}
	if data["token"] == nil || data["token"].(string) == "" {
		t.Errorf("Expected valid JWT token in auth response, got nil/empty")
	}

	// Boundary Scenario: empty code parameter
	emptyBody := g.Map{"code": ""}
	status, resp, err = sendRequest(http.MethodPost, "/v1/wechat/auth", emptyBody, "")
	if err != nil {
		t.Fatalf("WechatAuth empty request failed: %v", err)
	}
	// GoFrame validation will catch required field validation error (usually code 50/51/or custom error)
	if status == http.StatusOK && resp.Code == 0 {
		t.Errorf("Expected validation failure for empty code, but got success")
	}
}

// TestGetUserInfo tests /api/v1/user/info GET
func TestGetUserInfo(t *testing.T) {
	token, _ := GenerateTestToken(1, "test_openid_1", testConfig.JWTSecret)

	// Correct Scenario: authenticated request
	status, resp, err := sendRequest(http.MethodGet, "/v1/user/info", nil, token)
	if err != nil {
		t.Fatalf("GetUserInfo request failed: %v", err)
	}
	if status != http.StatusOK || resp.Code != 0 {
		t.Errorf("Expected 200, got status: %d, code: %d, msg: %s", status, resp.Code, resp.Message)
	}

	// Boundary Scenario: Access without token (401 Unauthorized)
	status, resp, err = sendRequest(http.MethodGet, "/v1/user/info", nil, "")
	if err != nil {
		t.Fatalf("GetUserInfo request without token failed: %v", err)
	}
	if status != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for empty token, got status: %d, msg: %s", status, resp.Message)
	}

	// Boundary Scenario: Access with invalid token format
	status, resp, err = sendRequest(http.MethodGet, "/v1/user/info", nil, "invalid-token-string")
	if err != nil {
		t.Fatalf("GetUserInfo request with invalid token failed: %v", err)
	}
	if status != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for malformed token, got status: %d, msg: %s", status, resp.Message)
	}
}

// TestUpdateUserInfo tests /api/v1/user/info PUT
func TestUpdateUserInfo(t *testing.T) {
	token, _ := GenerateTestToken(1, "test_openid_1", testConfig.JWTSecret)

	// Correct Scenario
	body := g.Map{
		"nickname":   "SuperDreamer",
		"avatar_url": "https://example.com/new-avatar.png",
	}
	status, resp, err := sendRequest(http.MethodPut, "/v1/user/info", body, token)
	if err != nil {
		t.Fatalf("UpdateUserInfo request failed: %v", err)
	}
	if status != http.StatusOK || resp.Code != 0 {
		t.Errorf("Expected 200, got status: %d, code: %d, msg: %s", status, resp.Code, resp.Message)
	}

	// Boundary Scenario: Update with empty token (401)
	status, _, err = sendRequest(http.MethodPut, "/v1/user/info", body, "")
	if err != nil {
		t.Fatalf("UpdateUserInfo unauthorized request failed: %v", err)
	}
	if status != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for empty token in PUT, got status: %d", status)
	}
}

// TestFetchDreamList tests /api/v1/dream/list
func TestFetchDreamList(t *testing.T) {
	token, _ := GenerateTestToken(1, "test_openid_1", testConfig.JWTSecret)

	// Correct Scenario
	status, resp, err := sendRequest(http.MethodGet, "/v1/dream/list?startDate=2026-05-01&endDate=2026-05-31&pageSize=5&page=1", nil, token)
	if err != nil {
		t.Fatalf("FetchDreamList request failed: %v", err)
	}
	if status != http.StatusOK || resp.Code != 0 {
		t.Errorf("Expected 200, got status: %d, code: %d, msg: %s", status, resp.Code, resp.Message)
	}

	// Boundary Scenario: Missing dates
	status, resp, err = sendRequest(http.MethodGet, "/v1/dream/list?pageSize=5&page=1", nil, token)
	if err != nil {
		t.Fatalf("FetchDreamList missing fields request failed: %v", err)
	}
	if status == http.StatusOK && resp.Code == 0 {
		t.Errorf("Expected error for missing dates, got success")
	}

	// Boundary Scenario: Invalid date format
	status, resp, err = sendRequest(http.MethodGet, "/v1/dream/list?startDate=invalid-date&endDate=2026-05-31", nil, token)
	if err != nil {
		t.Fatalf("FetchDreamList invalid date format request failed: %v", err)
	}
	if status == http.StatusOK && resp.Code == 0 {
		t.Errorf("Expected error for invalid date format, got success")
	}
}

// TestGetDreamAnalyzeResult tests /api/v1/dream/analyze/result
func TestGetDreamAnalyzeResult(t *testing.T) {
	token, _ := GenerateTestToken(1, "test_openid_1", testConfig.JWTSecret)

	// Correct Scenario: Retrieve analysis result for ID = 1 (seeded in mock service)
	status, resp, err := sendRequest(http.MethodGet, "/v1/dream/analyze/result?id=1", nil, token)
	if err != nil {
		t.Fatalf("GetDreamAnalyzeResult request failed: %v", err)
	}
	if status != http.StatusOK || resp.Code != 0 {
		t.Errorf("Expected 200, got status: %d, code: %d, msg: %s", status, resp.Code, resp.Message)
	}

	// Boundary Scenario: Query non-existent ID
	status, resp, err = sendRequest(http.MethodGet, "/v1/dream/analyze/result?id=9999", nil, token)
	if err != nil {
		t.Fatalf("GetDreamAnalyzeResult non-existent request failed: %v", err)
	}
	// Expected error code due to database missing record
	if status == http.StatusOK && resp.Code == 0 {
		t.Errorf("Expected error for non-existent ID=9999, got success")
	}

	// Boundary Scenario: Missing required query parameter 'id'
	status, resp, err = sendRequest(http.MethodGet, "/v1/dream/analyze/result", nil, token)
	if err != nil {
		t.Fatalf("GetDreamAnalyzeResult missing ID request failed: %v", err)
	}
	if status == http.StatusOK && resp.Code == 0 {
		t.Errorf("Expected validation failure for missing id, got success")
	}
}

// TestDeleteDream tests /api/v1/dream/delete
func TestDeleteDream(t *testing.T) {
	token, _ := GenerateTestToken(1, "test_openid_1", testConfig.JWTSecret)

	// Correct Scenario: Delete dream ID=1
	body := g.Map{"id": 1}
	status, resp, err := sendRequest(http.MethodPost, "/v1/dream/delete", body, token)
	if err != nil {
		t.Fatalf("DeleteDream request failed: %v", err)
	}
	if status != http.StatusOK || resp.Code != 0 {
		t.Errorf("Expected 200, got status: %d, code: %d, msg: %s", status, resp.Code, resp.Message)
	}

	// Verify deletion: attempting to delete it again should return success: false or error
	status, resp, err = sendRequest(http.MethodPost, "/v1/dream/delete", body, token)
	if err != nil {
		t.Fatalf("DeleteDream duplicate request failed: %v", err)
	}

	var data map[string]interface{}
	_ = json.Unmarshal(resp.Data, &data)
	if data["success"] == true {
		t.Errorf("Expected duplicate delete to return success: false, got true")
	}

	// Boundary Scenario: Missing 'id' parameter
	emptyBody := g.Map{}
	status, resp, err = sendRequest(http.MethodPost, "/v1/dream/delete", emptyBody, token)
	if err != nil {
		t.Fatalf("DeleteDream empty request failed: %v", err)
	}
	if status == http.StatusOK && resp.Code == 0 {
		t.Errorf("Expected error for missing id parameter, got success")
	}
}
