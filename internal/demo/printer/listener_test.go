package printer

import (
	"net"
	"testing"
	"time"

	"karots-suite/internal/escpos"
)

func send(t *testing.T, addr string, job []byte) {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	c.Write(job)
	c.Close()
}

func waitFor(t *testing.T, p *Printer, n int) []Print {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if got := p.Since(0); len(got) >= n {
			return got
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fewer than %d prints arrived", n)
	return nil
}

func TestListenerJob(t *testing.T) {
	p, err := Listen(Receipt)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	job, _ := escpos.ImageJob(pattern(384, 30), 384)
	send(t, p.Addr, job)
	got := waitFor(t, p, 1)
	if got[0].ID != 1 || got[0].Kind != Receipt || len(got[0].PNG) == 0 {
		t.Fatalf("print = %+v", got[0])
	}
	if len(p.Since(1)) != 0 {
		t.Fatal("Since(1) should be empty after one print")
	}
}

func TestRingKeeps20(t *testing.T) {
	p, err := Listen(Label)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for i := 0; i < 25; i++ {
		send(t, p.Addr, []byte("SIZE 50 mm, 30 mm\r\nTEXT 1,1,\"3\",0,1,1,\"x\"\r\nPRINT 1,1\r\n"))
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && p.last() < 25 {
		time.Sleep(10 * time.Millisecond)
	}
	got := p.Since(0)
	if len(got) != 20 || got[0].ID != 6 || got[19].ID != 25 {
		t.Fatalf("kept %d prints, first %d", len(got), got[0].ID)
	}
}
