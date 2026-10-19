package shopconf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample() Conf {
	return Conf{
		Version: Version, Shop: "Al Raj", Key: "al-raj", InstallID: "7C1E09A2", OS: "linux",
		Mode: "server", Binary: "karots-suite", InstallDir: "/opt/karots-suite", Port: 3000, Open: "kiosk",
		DB:  &DB{Host: "localhost", Port: 5432, Name: "suite_db", User: "suite_user", Password: "pw", SuperPassword: "sp", Local: true},
		Env: map[string]string{"SERVER_PORT": "3000"},
	}
}

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), FileName)
	if err := Write(p, sample()); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
	got, err := Read(p)
	if err != nil || !reflect.DeepEqual(got, sample()) {
		t.Fatalf("read back %+v (%v)", got, err)
	}
}

func TestReadRefuses(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(*Conf)
		want string
	}{
		"newer version": {func(c *Conf) { c.Version = 2 }, "rebuild in the Shop Builder"},
		"mode":          {func(c *Conf) { c.Mode = "x" }, "mode"},
		"server no db":  {func(c *Conf) { c.DB = nil }, "db"},
		"binary path":   {func(c *Conf) { c.Binary = "../x" }, "binary"},
		"port":          {func(c *Conf) { c.Port = 0 }, "port"},
		"open":          {func(c *Conf) { c.Open = "x" }, "open"},
		"env newline":   {func(c *Conf) { c.Env["A"] = "x\ny" }, "env"},
	} {
		c := sample()
		tc.edit(&c)
		p := filepath.Join(t.TempDir(), FileName)
		raw, _ := json.Marshal(c)
		os.WriteFile(p, raw, 0o600)
		if _, err := Read(p); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err %v, want it to mention %q", name, err, tc.want)
		}
	}
}

func TestEnvFile(t *testing.T) {
	c := Conf{Env: map[string]string{"B": "2", "A": "x y"}}
	if got := string(c.EnvFile()); got != "A=x y\nB=2\n" {
		t.Fatalf("got %q", got)
	}
}
