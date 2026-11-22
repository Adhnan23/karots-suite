package pocketcashier

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestPairApproveUse(t *testing.T) {
	en := newEnv(t)
	code, secret, _ := en.pairPhone()
	if len(code) != 6 {
		t.Fatalf("code %q", code)
	}
	if st := decodeAs[PairStatus](t, en.do(http.MethodGet, "/pair/status", nil, nil, secret)); st.Status != "pending" {
		t.Fatalf("status %+v", st)
	}
	if rec := en.do(http.MethodGet, "/whoami", nil, nil, secret); rec.Code < 400 {
		t.Fatalf("a pending phone got in: %d", rec.Code)
	}
	_, admin := en.user("admin")
	if rec := en.do(http.MethodPost, "/phones/approve", ApproveRequest{Code: code}, admin, ""); rec.Code >= 300 {
		t.Fatalf("approve %d %s", rec.Code, rec.Body)
	}
	if st := decodeAs[PairStatus](t, en.do(http.MethodGet, "/pair/status", nil, nil, secret)); st.Status != "active" {
		t.Fatalf("status %+v", st)
	}
	if rec := en.do(http.MethodGet, "/whoami", nil, nil, secret); rec.Code != http.StatusOK {
		t.Fatalf("active phone refused: %d %s", rec.Code, rec.Body)
	}
	if rec := en.do(http.MethodGet, "/whoami", nil, nil, "wrong-secret"); rec.Code != http.StatusUnauthorized || errCodeOf(rec) != "PHONE_UNKNOWN" {
		t.Fatalf("unknown phone: %d %s", rec.Code, rec.Body)
	}
	var stored []byte
	en.db.Get(&stored, `SELECT token_hash FROM pocket_phones WHERE pair_code IS NULL AND status = 'active' ORDER BY id DESC LIMIT 1`) //nolint:errcheck
	if string(stored) == secret {
		t.Fatal("raw secret stored")
	}
}

func TestBlockStopsAtOnce(t *testing.T) {
	en := newEnv(t)
	secret, device := en.activePhone()
	var id int64
	en.db.Get(&id, `SELECT id FROM pocket_phones WHERE device = $1`, device) //nolint:errcheck
	_, admin := en.user("admin")
	if rec := en.do(http.MethodPost, "/phones/"+strconv.FormatInt(id, 10)+"/block", nil, admin, ""); rec.Code >= 300 {
		t.Fatalf("block %d %s", rec.Code, rec.Body)
	}
	if rec := en.do(http.MethodGet, "/whoami", nil, nil, secret); rec.Code != http.StatusForbidden || errCodeOf(rec) != "PHONE_BLOCKED" {
		t.Fatalf("blocked phone: %d %s", rec.Code, rec.Body)
	}
}

func TestReleaseClearsDevice(t *testing.T) {
	en := newEnv(t)
	_, device := en.activePhone()
	uid, _ := en.user("cashier")
	var id int64
	en.db.Get(&id, `SELECT id FROM pocket_phones WHERE device = $1`, device) //nolint:errcheck
	en.db.MustExec(`UPDATE pocket_phones SET user_id = $1 WHERE id = $2`, uid, id)
	en.db.MustExec(`UPDATE users SET active_device = $1, active_device_label = 'Phone T' WHERE id = $2`, device, uid)
	_, admin := en.user("admin")
	if rec := en.do(http.MethodPost, "/phones/"+strconv.FormatInt(id, 10)+"/release", nil, admin, ""); rec.Code >= 300 {
		t.Fatalf("release %d %s", rec.Code, rec.Body)
	}
	var active *string
	var onPhone *int64
	en.db.Get(&active, `SELECT active_device FROM users WHERE id = $1`, uid)   //nolint:errcheck
	en.db.Get(&onPhone, `SELECT user_id FROM pocket_phones WHERE id = $1`, id) //nolint:errcheck
	if active != nil || onPhone != nil {
		t.Fatalf("still held: %v %v", active, onPhone)
	}
}

func TestCashierCannotApprove(t *testing.T) {
	en := newEnv(t)
	code, _, _ := en.pairPhone()
	for _, role := range []string{"cashier", "manager"} {
		_, ck := en.user(role)
		if rec := en.do(http.MethodPost, "/phones/approve", ApproveRequest{Code: code}, ck, ""); rec.Code != http.StatusForbidden {
			t.Fatalf("%s approve: %d %s", role, rec.Code, rec.Body)
		}
	}
}

func TestCAFiles(t *testing.T) {
	en := newEnv(t)
	for path, ct := range map[string]string{"/ca.crt": "application/x-x509-ca-cert", "/ca.pem": "application/x-pem-file", "/ca.mobileconfig": "application/x-apple-aspen-config"} {
		rec := en.do(http.MethodGet, path, nil, nil, "")
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != ct || rec.Body.Len() == 0 {
			t.Fatalf("%s: %d %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	if rec := en.do(http.MethodGet, "/ping", nil, nil, ""); rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("ping %d", rec.Code)
	}
}

// The phone asks whether its person moved to another device (they signed in on
// the PC), so it locks the way the PC does.
func TestMovedSaysWhere(t *testing.T) {
	en := newEnv(t)
	secret, device := en.activePhone()
	uid, _ := en.user("cashier")
	ask := func() Moved {
		return decodeAs[Moved](t, en.do(http.MethodGet, "/moved?user="+strconv.FormatInt(uid, 10), nil, nil, secret))
	}
	if m := ask(); m.MovedTo != "" {
		t.Fatalf("nobody's device yet: %+v", m)
	}
	en.db.Exec(`UPDATE users SET active_device = $2, active_device_label = 'Counter PC' WHERE id = $1`, uid, device) //nolint:errcheck
	if m := ask(); m.MovedTo != "" {
		t.Fatalf("still on this phone: %+v", m)
	}
	en.db.Exec(`UPDATE users SET active_device = 'pc-1', active_device_label = 'Counter PC' WHERE id = $1`, uid) //nolint:errcheck
	if m := ask(); m.MovedTo != "Counter PC" {
		t.Fatalf("moved: %+v", m)
	}
}

// A pairing request's limits, at their edges: a key of 32–200 characters, a name of
// up to 40, a device id of up to 64.
func TestPairLimits(t *testing.T) {
	en := newEnv(t)
	key := func(n int) string { return strings.Repeat("k", n) } // made unique per case below, at the same length
	for i, x := range []struct {
		name, device, secret string
		ok                   bool
	}{
		{"Phone", "dev-a", key(31), false}, {"Phone", "dev-b", key(32), true},
		{"Phone", "dev-c", key(200), true}, {"Phone", "dev-d", key(201), false},
		{strings.Repeat("n", 40), "dev-e", key(40), true}, {strings.Repeat("n", 41), "dev-f", key(41), false},
		{"Phone", strings.Repeat("d", 64), key(42), true}, {"Phone", strings.Repeat("d", 65), key(43), false},
	} {
		rec := en.do(http.MethodPost, "/pair", PairRequest{Name: x.name, Device: x.device, Secret: string(rune('a'+i)) + x.secret[1:]}, nil, "")
		if (rec.Code == http.StatusOK) != x.ok {
			t.Errorf("case %d (name %d, device %d, key %d): %d %s", i, len(x.name), len(x.device), len(x.secret), rec.Code, rec.Body)
		}
	}
}
