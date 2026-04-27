package facebook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
)

// integrationCredentials returns the token and page ID from environment variables,
// skipping the test if either is absent.
func integrationCredentials(t *testing.T) (token, pageID string) {
	t.Helper()
	token = os.Getenv("FACEBOOK_PAGE_TOKEN")
	pageID = os.Getenv("FACEBOOK_PAGE_ID")
	if token == "" || pageID == "" {
		t.Skip("set FACEBOOK_PAGE_TOKEN and FACEBOOK_PAGE_ID to run Facebook integration tests")
	}
	return
}

// graphGet performs a GET request against the Graph API and returns the parsed JSON body.
// It fails the test immediately on any network error, non-200 status, or Graph API error field.
func graphGet(t *testing.T, path, token string, extra url.Values) map[string]interface{} {
	t.Helper()
	u := fmt.Sprintf("https://graph.facebook.com/v22.0/%s", path)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		t.Fatalf("build request for %s: %v", path, err)
	}
	q := req.URL.Query()
	q.Set("access_token", token)
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	req.URL.RawQuery = q.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", u, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: HTTP %d: %s", path, resp.StatusCode, body)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("GET %s: JSON decode: %v\nbody: %s", path, err, body)
	}
	if errField, ok := result["error"]; ok {
		t.Fatalf("GET %s: Graph API error: %v", path, errField)
	}
	return result
}

// tokenScopes returns the OAuth scopes granted to the given token via the debug_token endpoint.
func tokenScopes(t *testing.T, token string) []string {
	t.Helper()
	result := graphGet(t, "debug_token", token, url.Values{"input_token": {token}})
	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("debug_token: unexpected response shape: %v", result)
	}
	raw, ok := data["scopes"].([]interface{})
	if !ok {
		t.Fatalf("debug_token: missing scopes array: %v", data)
	}
	scopes := make([]string, len(raw))
	for i, s := range raw {
		scopes[i] = fmt.Sprint(s)
	}
	return scopes
}

func containsScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == want {
			return true
		}
	}
	return false
}

// TestIntegrationPublicProfile verifies the public_profile permission by reading the page's
// id and name fields.
func TestIntegrationPublicProfile(t *testing.T) {
	token, pageID := integrationCredentials(t)
	result := graphGet(t, pageID, token, url.Values{"fields": {"id,name"}})
	if result["id"] == nil {
		t.Errorf("public_profile: response missing 'id' field: %v", result)
	}
	if result["name"] == nil {
		t.Errorf("public_profile: response missing 'name' field: %v", result)
	}
}

// TestIntegrationPagesShowList verifies the pages_show_list permission by requesting the
// list of pages accessible via the token.
func TestIntegrationPagesShowList(t *testing.T) {
	token, _ := integrationCredentials(t)
	result := graphGet(t, "me/accounts", token, nil)
	if result["data"] == nil {
		t.Errorf("pages_show_list: response missing 'data' field: %v", result)
	}
}

// TestIntegrationPagesReadUserContent verifies the pages_read_user_content permission by
// fetching up to 5 posts from the page's feed.
func TestIntegrationPagesReadUserContent(t *testing.T) {
	token, pageID := integrationCredentials(t)
	result := graphGet(t, pageID+"/posts", token, url.Values{"limit": {"5"}})
	if result["data"] == nil {
		t.Errorf("pages_read_user_content: response missing 'data' field: %v", result)
	}
}

// TestIntegrationPagesReadEngagement verifies the pages_read_engagement permission by
// reading fan_count and followers_count fields, which are gated behind this scope.
func TestIntegrationPagesReadEngagement(t *testing.T) {
	token, pageID := integrationCredentials(t)
	result := graphGet(t, pageID, token, url.Values{"fields": {"fan_count,followers_count"}})
	_, hasFanCount := result["fan_count"]
	_, hasFollowers := result["followers_count"]
	if !hasFanCount && !hasFollowers {
		t.Errorf("pages_read_engagement: expected fan_count or followers_count fields: %v", result)
	}
}

// TestIntegrationPagesManagePosts verifies the pages_manage_posts permission is granted on
// the token without creating or modifying any content.
func TestIntegrationPagesManagePosts(t *testing.T) {
	token, _ := integrationCredentials(t)
	scopes := tokenScopes(t, token)
	if !containsScope(scopes, "pages_manage_posts") {
		t.Errorf("pages_manage_posts: scope not present; token has scopes: %v", scopes)
	}
}

// TestIntegrationBusinessManagement verifies the business_management permission is granted
// on the token without modifying any business data.
func TestIntegrationBusinessManagement(t *testing.T) {
	token, _ := integrationCredentials(t)
	scopes := tokenScopes(t, token)
	if !containsScope(scopes, "business_management") {
		t.Errorf("business_management: scope not present; token has scopes: %v", scopes)
	}
}
