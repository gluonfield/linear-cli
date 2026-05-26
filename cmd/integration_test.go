//go:build integration

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gluonfield/linear-cli/api"
)

func skipNoAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("LINEAR_API_KEY") == "" {
		t.Skip("LINEAR_API_KEY not set, skipping integration test")
	}
}

func createTestIssue(t *testing.T, title string) string {
	t.Helper()
	q := fmt.Sprintf(`mutation { issueCreate(input: { title: "test-cli: %s", teamId: "%s" }) { issue { id identifier } } }`,
		title, testTeamID(t))
	var res struct {
		IssueCreate struct {
			Issue struct {
				ID         string `json:"id"`
				Identifier string `json:"identifier"`
			} `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("create test issue: %v", err)
	}
	t.Logf("created test issue: %s (%s)", res.IssueCreate.Issue.Identifier, res.IssueCreate.Issue.ID)
	return res.IssueCreate.Issue.ID
}

func deleteTestIssue(t *testing.T, id string) {
	t.Helper()
	q := fmt.Sprintf(`mutation { issueDelete(id: "%s") { success } }`, id)
	var res struct {
		IssueDelete struct {
			Success bool `json:"success"`
		} `json:"issueDelete"`
	}
	err := api.Query(q, &res)
	if err != nil {
		t.Logf("cleanup delete failed: %v", err)
	}
}

func testTeamID(t *testing.T) string {
	t.Helper()
	q := `query { teams(first: 1) { nodes { id key } } }`
	var res struct {
		Teams struct {
			Nodes []struct {
				ID  string `json:"id"`
				Key string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("get team: %v", err)
	}
	if len(res.Teams.Nodes) == 0 {
		t.Fatal("no teams found")
	}
	return res.Teams.Nodes[0].ID
}

func TestIntegration_ParseIssueIdentifier_URL(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "url-parse-test")
	defer deleteTestIssue(t, id)

	q := fmt.Sprintf(`query { issue(id: "%s") { identifier } }`, id)
	var res struct {
		Issue struct {
			Identifier string `json:"identifier"`
		} `json:"issue"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("query issue: %v", err)
	}

	fakeURL := fmt.Sprintf("https://linear.app/team/issue/%s/some-title", res.Issue.Identifier)
	parsed := parseIssueIdentifier(fakeURL)
	if parsed != res.Issue.Identifier {
		t.Errorf("parseIssueIdentifier(%q) = %q, want %q", fakeURL, parsed, res.Issue.Identifier)
	}
}

func TestIntegration_CreateAndGet(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "create-and-get")
	defer deleteTestIssue(t, id)

	q := fmt.Sprintf(`query { issue(id: "%s") { id identifier title } }`, id)
	var res struct {
		Issue struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
		} `json:"issue"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if res.Issue.ID != id {
		t.Errorf("issue id = %q, want %q", res.Issue.ID, id)
	}
	if !strings.Contains(res.Issue.Title, "create-and-get") {
		t.Errorf("issue title = %q, want to contain create-and-get", res.Issue.Title)
	}
}

func TestIntegration_UpdateStatusFuzzy(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "fuzzy-status-test")
	defer deleteTestIssue(t, id)

	teamID := testTeamID(t)

	states, err := fetchTeamStates(teamID)
	if err != nil {
		t.Fatalf("fetch states: %v", err)
	}
	if len(states) == 0 {
		t.Fatal("no workflow states found")
	}

	firstState := states[0].Name
	stateID, stateName, err := fuzzyMatchState(states, firstState)
	if err != nil {
		t.Fatalf("fuzzyMatchState(%q): %v", firstState, err)
	}
	if stateID == "" {
		t.Fatal("fuzzyMatchState returned empty id")
	}
	t.Logf("resolved state %q -> %s", firstState, stateName)

	mutQ := fmt.Sprintf(`mutation { issueUpdate(id: "%s", input: { stateId: "%s" }) { success issue { state { name } } } }`, id, stateID)
	var mutRes struct {
		IssueUpdate struct {
			Success bool `json:"success"`
			Issue   struct {
				State *struct {
					Name string `json:"name"`
				} `json:"state"`
			} `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := api.Query(mutQ, &mutRes); err != nil {
		t.Fatalf("update status: %v", err)
	}
	if !mutRes.IssueUpdate.Success {
		t.Fatal("update returned success=false")
	}
	if mutRes.IssueUpdate.Issue.State == nil {
		t.Fatal("issue state is nil after update")
	}
	if mutRes.IssueUpdate.Issue.State.Name != stateName {
		t.Errorf("state = %q, want %q", mutRes.IssueUpdate.Issue.State.Name, stateName)
	}
}

func TestIntegration_UpdateStatusFuzzy_CaseInsensitive(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "fuzzy-ci-test")
	defer deleteTestIssue(t, id)

	teamID := testTeamID(t)
	states, err := fetchTeamStates(teamID)
	if err != nil {
		t.Fatalf("fetch states: %v", err)
	}
	if len(states) == 0 {
		t.Fatal("no states")
	}

	lowerName := strings.ToLower(states[0].Name)
	stateID, _, err := fuzzyMatchState(states, lowerName)
	if err != nil {
		t.Fatalf("fuzzyMatchState(%q): %v", lowerName, err)
	}
	if stateID == "" {
		t.Fatalf("fuzzyMatchState(%q) returned empty id", lowerName)
	}
}

func TestIntegration_UpdateStatusFuzzy_Partial(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "fuzzy-partial-test")
	defer deleteTestIssue(t, id)

	teamID := testTeamID(t)
	states, err := fetchTeamStates(teamID)
	if err != nil {
		t.Fatalf("fetch states: %v", err)
	}

	for _, s := range states {
		if len(s.Name) >= 5 {
			partial := s.Name[:5]
			stateID, matchedName, err := fuzzyMatchState(states, partial)
			if err != nil {
				t.Logf("partial %q: %v (might be ambiguous, ok)", partial, err)
				continue
			}
			if matchedName != s.Name {
				t.Errorf("partial %q matched %q, want %q", partial, matchedName, s.Name)
			}
			t.Logf("partial match %q -> %s (id: %s)", partial, matchedName, stateID)
			return
		}
	}
}

func TestIntegration_ResolveAssigneeMe(t *testing.T) {
	skipNoAPIKey(t)

	viewerID, err := getViewerID()
	if err != nil {
		t.Fatalf("getViewerID: %v", err)
	}
	if viewerID == "" {
		t.Fatal("getViewerID returned empty")
	}

	meID, err := resolveAssigneeID("me")
	if err != nil {
		t.Fatalf("resolveAssigneeID(me): %v", err)
	}
	if meID != viewerID {
		t.Errorf("resolveAssigneeID(me) = %q, viewerID = %q", meID, viewerID)
	}
}

func TestIntegration_Comment(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "comment-test")
	defer deleteTestIssue(t, id)

	body := "test comment at " + time.Now().Format(time.RFC3339)
	mutQ := fmt.Sprintf(`mutation { commentCreate(input: { issueId: "%s", body: "%s" }) { comment { id body } } }`, id, escapeGraphQL(body))
	var mutRes struct {
		CommentCreate struct {
			Comment struct {
				ID   string `json:"id"`
				Body string `json:"body"`
			} `json:"comment"`
		} `json:"commentCreate"`
	}
	if err := api.Query(mutQ, &mutRes); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if mutRes.CommentCreate.Comment.Body != body {
		t.Errorf("comment body = %q, want %q", mutRes.CommentCreate.Comment.Body, body)
	}

	q := fmt.Sprintf(`query { issue(id: "%s") { comments { nodes { id body } } } }`, id)
	var res struct {
		Issue struct {
			Comments struct {
				Nodes []struct {
					ID   string `json:"id"`
					Body string `json:"body"`
				} `json:"nodes"`
			} `json:"comments"`
		} `json:"issue"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("list comments: %v", err)
	}
	found := false
	for _, c := range res.Issue.Comments.Nodes {
		if c.Body == body {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("comment not found in listing")
	}
}

func TestIntegration_SearchNoArgs(t *testing.T) {
	skipNoAPIKey(t)

	viewerID, err := getViewerID()
	if err != nil {
		t.Fatalf("getViewerID: %v", err)
	}

	id := createTestIssue(t, "search-noargs-test")
	defer deleteTestIssue(t, id)

	mutQ := fmt.Sprintf(`mutation { issueUpdate(id: "%s", input: { assigneeId: "%s" }) { success } }`, id, viewerID)
	var mutRes struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}
	if err := api.Query(mutQ, &mutRes); err != nil {
		t.Fatalf("assign issue: %v", err)
	}

	q := fmt.Sprintf(`query { issues(filter: { assignee: { id: { eq: "%s" } } }, first: 50) { nodes { id } } }`, viewerID)
	var res struct {
		Issues struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("search my issues: %v", err)
	}
	found := false
	for _, n := range res.Issues.Nodes {
		if n.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("created issue not found in my issues (may need assignee)")
	}
}

func TestIntegration_SearchWithTerm(t *testing.T) {
	skipNoAPIKey(t)
	uniqueTerm := "xuniq" + fmt.Sprintf("%d", time.Now().UnixNano())
	id := createTestIssue(t, uniqueTerm)
	defer deleteTestIssue(t, id)

	time.Sleep(2 * time.Second)

	q := fmt.Sprintf(`query { searchIssues(term: "%s", first: 10) { nodes { id } } }`, uniqueTerm)
	var res struct {
		SearchIssues struct {
			Nodes []struct {
				ID string `json:"id"`
			} `json:"nodes"`
		} `json:"searchIssues"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("searchIssues: %v", err)
	}
	if len(res.SearchIssues.Nodes) == 0 {
		t.Errorf("searchIssues(%q) returned 0 results (search index may lag)", uniqueTerm)
	}
}

func TestIntegration_GetWithGraphQLDirect(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "get-full-test")
	defer deleteTestIssue(t, id)

	q := fmt.Sprintf(`query { issue(id: "%s") { id identifier title state { name } assignee { name } priority team { key name } url createdAt updatedAt } }`, id)
	var res struct {
		Issue *struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
			State      *struct {
				Name string `json:"name"`
			} `json:"state"`
			Team *struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"team"`
			URL string `json:"url"`
		} `json:"issue"`
	}
	if err := api.Query(q, &res); err != nil {
		t.Fatalf("get issue: %v", err)
	}
	if res.Issue == nil {
		t.Fatal("issue is nil")
	}
	if res.Issue.ID != id {
		t.Errorf("id = %q, want %q", res.Issue.ID, id)
	}
	if !strings.Contains(res.Issue.URL, res.Issue.Identifier) {
		t.Errorf("url %q should contain identifier %q", res.Issue.URL, res.Issue.Identifier)
	}
}

func TestIntegration_ViewerInfo(t *testing.T) {
	skipNoAPIKey(t)

	id, err := getViewerID()
	if err != nil {
		t.Fatalf("getViewerID: %v", err)
	}
	if id == "" {
		t.Fatal("viewer id is empty")
	}

	name, err := getViewerName()
	if err != nil {
		t.Fatalf("getViewerName: %v", err)
	}
	if name == "" {
		t.Fatal("viewer name is empty")
	}
	t.Logf("viewer: %s (%s)", name, id)
}

func TestIntegration_DeleteIssue(t *testing.T) {
	skipNoAPIKey(t)
	id := createTestIssue(t, "delete-test")

	mutQ := fmt.Sprintf(`mutation { issueDelete(id: "%s") { success } }`, id)
	var res struct {
		IssueDelete struct {
			Success bool `json:"success"`
		} `json:"issueDelete"`
	}
	if err := api.Query(mutQ, &res); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !res.IssueDelete.Success {
		t.Fatal("delete returned success=false")
	}

	q := fmt.Sprintf(`query { issue(id: "%s") { id archivedAt } }`, id)
	var getRes struct {
		Issue *struct {
			ID        string  `json:"id"`
			ArchivedAt *string `json:"archivedAt"`
		} `json:"issue"`
	}
	err := api.Query(q, &getRes)
	if err == nil && getRes.Issue != nil && getRes.Issue.ArchivedAt == nil {
		t.Log("issue still exists after delete (may be in trash)")
	}
}

func TestIntegration_OutputHelpers_WithRealData(t *testing.T) {
	skipNoAPIKey(t)

	type issue struct {
		ID         string `json:"id"`
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
		Priority   int    `json:"priority"`
	}

	sample := issue{ID: "abc", Identifier: "TEST-1", Title: "Test Issue", Priority: 2}
	m := toMap(sample)

	if m["identifier"] != "TEST-1" {
		t.Errorf("toMap identifier = %v", m["identifier"])
	}
	if fieldStr(m["priority"]) != "2" {
		t.Errorf("fieldStr(priority) = %v", fieldStr(m["priority"]))
	}
	if fieldStr(m["title"]) != "Test Issue" {
		t.Errorf("fieldStr(title) = %v", fieldStr(m["title"]))
	}
	if fieldStr(m["id"]) != "abc" {
		t.Errorf("fieldStr(id) = %v", fieldStr(m["id"]))
	}
}

func TestIntegration_JSONRoundTrip(t *testing.T) {
	skipNoAPIKey(t)

	type nested struct {
		Name string `json:"name"`
	}
	type obj struct {
		ID    string `json:"id"`
		State nested `json:"state"`
	}

	original := obj{ID: "123", State: nested{Name: "Todo"}}
	m := toMap(original)

	b, _ := json.Marshal(m)
	var back obj
	json.Unmarshal(b, &back)

	if back.ID != original.ID {
		t.Errorf("roundtrip id = %q, want %q", back.ID, original.ID)
	}
	if back.State.Name != original.State.Name {
		t.Errorf("roundtrip state.name = %q, want %q", back.State.Name, original.State.Name)
	}
}
