package printer

import (
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Kind is which printer a job went to.
type Kind string

const (
	Receipt Kind = "receipt"
	Label   Kind = "label"
)

// Print is one job as the paper strip shows it.
type Print struct {
	ID   int64 `json:"id"`
	Kind Kind  `json:"kind"`
	Paper
	At time.Time `json:"at"`
}

const keep = 60 // the Prints shelf scrolls back through these

// Printer is one emulated network printer on 127.0.0.1.
type Printer struct {
	Addr string // host:port — the shop's settings use tcp://<Addr>
	kind Kind
	ln   net.Listener

	mu     sync.Mutex
	prints []Print
	nextID int64
}

// Listen opens an emulated printer on a free local port.
func Listen(kind Kind) (*Printer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	p := &Printer{Addr: ln.Addr().String(), kind: kind, ln: ln}
	go p.serve()
	return p, nil
}

func (p *Printer) serve() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return // closed
		}
		go p.job(c)
	}
}

// job reads one connection to EOF (the suite closes after writing) — or 2 s of
// silence — as one print.
func (p *Printer) job(c net.Conn) {
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	raw, _ := io.ReadAll(c)
	if len(raw) == 0 {
		return
	}
	var paper Paper
	var err error
	if p.kind == Label {
		paper, err = DecodeTSPL(raw)
	} else {
		paper, err = DecodeESCPOS(raw)
	}
	if err != nil {
		log.Printf("demo %s printer: %v", p.kind, err)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextID++
	p.prints = append(p.prints, Print{ID: p.nextID, Kind: p.kind, Paper: paper, At: time.Now()})
	if len(p.prints) > keep {
		p.prints = p.prints[len(p.prints)-keep:]
	}
}

// Since returns the kept prints with ID > id, oldest first.
func (p *Printer) Since(id int64) []Print {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Print
	for _, pr := range p.prints {
		if pr.ID > id {
			out = append(out, pr)
		}
	}
	return out
}

func (p *Printer) last() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.nextID
}

func (p *Printer) Kind() Kind { return p.kind }

func (p *Printer) Close() error { return p.ln.Close() }
