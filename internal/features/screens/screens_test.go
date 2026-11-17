package screens

import (
	"context"
	"os"
	"testing"
	"time"

	appdb "karots-suite/internal/db"

	"github.com/lib/pq"
)

// Sizes no real till has, removed before and after, so the dev database keeps its own rows.
func TestSeenAndList(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	conn, err := appdb.Connect(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx := context.Background()
	clean := func() {
		conn.MustExec(`DELETE FROM screen_sizes WHERE (w, h) IN ((1111, 777), (1112, 778), (1113, 779))`)
	}
	clean()
	defer clean()

	s := New(conn)
	now := time.Now()
	s.Seen(ctx, "dev-a", 1111, 777, "Shehan", now)
	s.Seen(ctx, "dev-b", 1111, 777, "Nimal", now)
	s.Seen(ctx, "dev-b", 1111, 777, "Nimal", now.Add(time.Minute)) // same device, same size, within the hour: no write
	s.Seen(ctx, "dev-c", 120, 8000, "Kamal", now)                  // not a real screen: ignored

	var users pq.StringArray
	var seen time.Time
	if err := conn.QueryRowxContext(ctx, `SELECT users, last_seen FROM screen_sizes WHERE w = 1111 AND h = 777`).Scan(&users, &seen); err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0] != "Shehan" || users[1] != "Nimal" {
		t.Fatalf("users = %v", users)
	}
	if seen.Sub(now).Abs() > time.Second {
		t.Fatalf("a repeat beat inside the hour wrote again: last_seen %v", seen)
	}

	// An hour later the same device writes again (last_seen moves on).
	s.Seen(ctx, "dev-b", 1111, 777, "Nimal", now.Add(61*time.Minute))
	if err := conn.GetContext(ctx, &seen, `SELECT last_seen FROM screen_sizes WHERE w = 1111 AND h = 777`); err != nil {
		t.Fatal(err)
	}
	if seen.Sub(now.Add(61*time.Minute)).Abs() > time.Second {
		t.Fatalf("an hour later it should write: last_seen %v", seen)
	}

	// A device that changes size writes at once.
	s.Seen(ctx, "dev-a", 1112, 778, "Shehan", now.Add(2*time.Minute))

	// Last seen 91 days ago: not listed.
	conn.MustExec(`INSERT INTO screen_sizes (w, h, users, last_seen) VALUES (1113, 779, '{Old}', $1)`, now.Add(-91*24*time.Hour))

	list, err := s.List(ctx, now.Add(62*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	got := map[[2]int]Size{}
	for _, z := range list {
		got[[2]int{z.W, z.H}] = z
	}
	if _, ok := got[[2]int{1111, 777}]; !ok {
		t.Fatal("1111x777 missing")
	}
	if _, ok := got[[2]int{1112, 778}]; !ok {
		t.Fatal("a changed size should be written at once")
	}
	if _, ok := got[[2]int{1113, 779}]; ok {
		t.Fatal("a size unseen for 91 days is listed")
	}
	if _, ok := got[[2]int{120, 8000}]; ok {
		t.Fatal("a junk size is listed")
	}
}

func TestKeepsLastSixNames(t *testing.T) {
	got := addName([]string{"a", "b", "c", "d", "e", "f"}, "g")
	if len(got) != 6 || got[0] != "b" || got[5] != "g" {
		t.Fatalf("got %v", got)
	}
	if got := addName([]string{"a", "b"}, "a"); len(got) != 2 {
		t.Fatalf("a name already there is added again: %v", got)
	}
}
