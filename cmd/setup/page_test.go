package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const tok = "0123456789abcdef0123456789abcdef"

func testPage(steps func(*Job) []Step) http.Handler {
	mk := func(ch Choice) (*Job, error) {
		j, err := applyChoice(conf("linux", "file", false), "", ch)
		if j != nil {
			j.Run, j.Log = &rec{}, io.Discard
		}
		return j, err
	}
	return pageHandler(tok, mk, steps, func() {})
}

func call(h http.Handler, method, path, body string, token bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token {
		req.Header.Set("X-Setup-Token", tok)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPageNeedsToken(t *testing.T) {
	h := testPage(Steps)
	for _, p := range []string{"/", "/api/plan?role=server"} {
		if rec := call(h, "GET", p, "", false); rec.Code != http.StatusForbidden {
			t.Errorf("%s without token: %d", p, rec.Code)
		}
		if rec := call(h, "GET", p+"&t=wrong", "", false); rec.Code != http.StatusForbidden && p != "/" {
			t.Errorf("%s wrong token: %d", p, rec.Code)
		}
	}
	if rec := call(h, "GET", "/?t="+tok, "", false); rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>") {
		t.Fatalf("page with token: %d", rec.Code)
	}
}

func TestPagePlan(t *testing.T) {
	rec := call(testPage(Steps), "GET", "/api/plan?role=server", "", true)
	var p struct {
		Shop  string
		Steps []struct{ ID, Title, Detail string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Shop != "Al Raj" || len(p.Steps) == 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	for _, s := range p.Steps {
		if s.ID == "database" {
			t.Fatal("file mode plans a database step")
		}
		if s.Detail == "" {
			t.Fatalf("step %s has no detail", s.ID)
		}
	}
}

func TestPageRunStreams(t *testing.T) {
	steps := func(*Job) []Step {
		return []Step{
			{ID: "a", Title: "A", Do: func(j *Job) error { fmt.Fprintln(j.Log, "doing a"); return nil }},
			{ID: "b", Title: "B", Do: func(*Job) error { return nil }},
		}
	}
	body := call(testPage(steps), "POST", "/api/run", `{"role":"server"}`, true).Body.String()
	for _, want := range []string{"@@STEP a run", "doing a", "@@STEP a ok", "@@STEP b ok", "@@DONE"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	need := func(*Job) []Step {
		return []Step{{ID: "database", Title: "Database", Do: func(*Job) error { return ErrNeedSuperPassword }}}
	}
	body = call(testPage(need), "POST", "/api/run", `{"role":"server"}`, true).Body.String()
	if !strings.Contains(body, "@@STEP database fail") || !strings.Contains(body, "@@NEED superPassword") || strings.Contains(body, "@@DONE") {
		t.Fatalf("need:\n%s", body)
	}
}

func TestPageRefusesTextPlain(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/run", strings.NewReader(`{"role":"server"}`))
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("X-Setup-Token", tok)
	rec := httptest.NewRecorder()
	testPage(Steps).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("%d", rec.Code)
	}
}

// The person at the PC picks kiosk or background; the plan follows it.
func TestPagePlanOpenChoice(t *testing.T) {
	var p struct {
		Open  string
		Steps []struct{ ID, Detail string }
	}
	rec := call(testPage(Steps), "GET", "/api/plan?role=server&open=background", "", true)
	json.Unmarshal(rec.Body.Bytes(), &p)
	if p.Open != "background" {
		t.Fatalf("open %q: %s", p.Open, rec.Body)
	}
	for _, s := range p.Steps {
		if s.ID == "open" && !strings.Contains(s.Detail, "background") {
			t.Fatalf("open step: %q", s.Detail)
		}
	}
}

// The page's folder browser: list a folder's sub-folders, go up, make a new one.
func TestPageFolders(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"b", "a", ".hidden"} {
		os.Mkdir(filepath.Join(dir, d), 0o755)
	}
	os.WriteFile(filepath.Join(dir, "file.txt"), nil, 0o644)
	h := testPage(Steps)
	rec := call(h, "GET", "/api/folders?path="+url.QueryEscape(dir), "", true)
	var f struct {
		Path, Parent string
		Folders      []string
		Roots        []struct{ Name, Path string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil || f.Path != dir || f.Parent != filepath.Dir(dir) || strings.Join(f.Folders, ",") != "a,b" || len(f.Roots) == 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// A folder that doesn't exist yet opens at its nearest existing parent.
	rec = call(h, "GET", "/api/folders?path="+url.QueryEscape(filepath.Join(dir, "a", "not", "yet")), "", true)
	json.Unmarshal(rec.Body.Bytes(), &f)
	if f.Path != filepath.Join(dir, "a") {
		t.Fatalf("missing folder opened at %q", f.Path)
	}
	if rec := call(h, "POST", "/api/folders/new", fmt.Sprintf(`{"path":%q,"name":"Karots backups"}`, dir), true); rec.Code != 200 {
		t.Fatalf("new folder: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "Karots backups")); err != nil {
		t.Fatal("folder not made")
	}
	if rec := call(h, "POST", "/api/folders/new", fmt.Sprintf(`{"path":%q,"name":"../escape"}`, dir), true); rec.Code != http.StatusBadRequest {
		t.Fatalf("../ name: %d", rec.Code)
	}
}

// How often and how many to keep are picked on the page too.
func TestPageBackupChoices(t *testing.T) {
	rec := call(testPage(Steps), "GET", "/api/plan?role=server&backupEvery=3h&backupKeep=56", "", true)
	var p struct {
		BackupEvery string
		BackupKeep  int
	}
	json.Unmarshal(rec.Body.Bytes(), &p)
	if p.BackupEvery != "3h" || p.BackupKeep != 56 {
		t.Fatalf("%s", rec.Body)
	}
	if rec := call(testPage(Steps), "GET", "/api/plan?role=server&backupEvery=5m", "", true); rec.Code != http.StatusBadRequest {
		t.Fatalf("every 5m: %d", rec.Code)
	}
}

func TestPageExitStopsSetup(t *testing.T) {
	quit := make(chan struct{}, 1)
	mk := func(ch Choice) (*Job, error) { return applyChoice(conf("linux", "file", false), "", ch) }
	h := pageHandler(tok, mk, Steps, func() { quit <- struct{}{} })
	if rec := call(h, "POST", "/api/close", `{}`, true); rec.Code != 200 {
		t.Fatalf("%d", rec.Code)
	}
	select {
	case <-quit:
	case <-time.After(time.Second):
		t.Fatal("Exit didn't stop Setup")
	}
	if rec := call(h, "POST", "/api/close", `{}`, false); rec.Code != http.StatusForbidden {
		t.Fatalf("Exit without the token: %d", rec.Code)
	}
}

// Review Focus 5: a mode change only runs when the person asked for it.
func TestCheckChoiceModeChangeNeedsConfirm(t *testing.T) {
	j, _ := applyChoice(conf("linux", "file", false), "", Choice{})
	j.Updating, j.FromMode = true, "server"
	if err := checkChoice(j); err == nil || !strings.Contains(err.Error(), "Server → One PC") || !strings.Contains(err.Error(), "-change-mode") {
		t.Fatalf("err %v", err)
	}
	j.ChangeMode = true
	if err := checkChoice(j); err != nil {
		t.Fatal(err)
	}
	j.ChangeMode, j.Uninstall = false, true
	if err := checkChoice(j); err != nil {
		t.Fatalf("uninstall refused: %v", err)
	}
}

func TestCheckChoiceRestoreOnUpdateRefused(t *testing.T) {
	j, _ := applyChoice(conf("linux", "file", false), "", Choice{Restore: "/media/usb/x.json.gz"})
	j.Updating = true
	if err := checkChoice(j); err == nil || !strings.Contains(err.Error(), "Data page") {
		t.Fatalf("err %v", err)
	}
	if _, err := applyChoice(conf("linux", "file", false), "", Choice{KeepDir: "backups"}); err == nil {
		t.Fatal("relative backups folder accepted")
	}
}

func TestPlanListsBackupsAndFromMode(t *testing.T) {
	keep := t.TempDir()
	os.WriteFile(filepath.Join(keep, "al-raj-final.json.gz"), []byte("x"), 0o644)
	mk := func(ch Choice) (*Job, error) {
		j, err := applyChoice(conf("linux", "server", true), t.TempDir(), ch)
		if j != nil {
			j.Run, j.Log, j.Updating, j.FromMode, j.KeepDir = &rec{}, io.Discard, true, "file", keep
		}
		return j, err
	}
	rec := call(pageHandler(tok, mk, Steps, func() {}), "GET", "/api/plan?role=server", "", true)
	var p struct {
		FromMode, KeepDir string
		LocalDB           bool `json:"localDb"`
		Backups           []struct{ Path, Name string }
	}
	json.Unmarshal(rec.Body.Bytes(), &p)
	if p.FromMode != "file" || p.KeepDir != keep || !p.LocalDB || len(p.Backups) != 1 || p.Backups[0].Name != "al-raj-final.json.gz" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// Not confirmed: the run is refused before any step.
	if r := call(pageHandler(tok, mk, Steps, func() {}), "POST", "/api/run", `{"role":"server"}`, true); r.Code != http.StatusBadRequest || !strings.Contains(r.Body.String(), "One PC → Server") {
		t.Fatalf("unconfirmed run: %d %s", r.Code, r.Body)
	}
}

func TestPlanUninstallSteps(t *testing.T) {
	rec := call(testPage(Steps), "GET", "/api/plan?role=server&uninstall=true", "", true)
	if !strings.Contains(rec.Body.String(), `"id":"final-backup"`) || strings.Contains(rec.Body.String(), `"id":"program"`) {
		t.Fatalf("%s", rec.Body)
	}
}

// "Bring back a shop" picks a backup file with the same browser: it lists the
// backups in a folder, nothing else.
func TestFoldersListBackupFiles(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b-final.json.gz", "a.json.gz", "notes.txt", "x.json.gz.tmp"} {
		os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644)
	}
	rec := call(testPage(Steps), "GET", "/api/folders?path="+url.QueryEscape(dir), "", true)
	var f struct{ Files []string }
	json.Unmarshal(rec.Body.Bytes(), &f)
	if strings.Join(f.Files, ",") != "a.json.gz,b-final.json.gz" {
		t.Fatalf("%s", rec.Body)
	}
}
