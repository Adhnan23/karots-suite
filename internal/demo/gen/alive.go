package gen

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"karots-suite/internal/demo/shops"
)

// Sim keeps a demo shop busy while the owner shows it: the staff on their tills
// sell through the shop's real services, beat presence so Who's on now shows
// them at work, take in a repair or a reload now and then, and a delivery comes
// when shelves run low. It never touches the owner's own logins.
type Sim struct {
	g      *gen
	mu     sync.Mutex
	paused bool
	staff  []person
	tick   time.Duration // the gap between sales at volume 1.0 (tests shorten it)
}

// NewSim prepares the simulator for a built shop (after Today).
func NewSim(ctx context.Context, d Deps, p shops.Profile) (*Sim, error) {
	g, err := newGen(ctx, d, p, time.Now())
	if err != nil {
		return nil, err
	}
	g.r = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 7)) // live: no need to repeat
	if g.s, err = loadShop(ctx, d, p); err != nil {
		return nil, err
	}
	if err := g.loadStock(); err != nil {
		return nil, err
	}
	if g.admin, err = NewClient(d.API, p.Logins[0].Phone, p.Logins[0].PIN, "demo-back-office"); err != nil {
		return nil, err
	}
	s := &Sim{g: g, staff: g.s.cashiers[1:], tick: time.Duration(float64(12*time.Hour) / math.Max(float64(p.Volume), 1))}
	g.open = s.staff
	return s, nil
}

func (s *Sim) Pause(on bool) {
	s.mu.Lock()
	s.paused = on
	s.mu.Unlock()
}

func (s *Sim) isPaused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// Run sells until ctx ends: presence every 20 s, a sale every so often (busier
// at lunch and in the evening), now and then something a plugin does.
func (s *Sim) Run(ctx context.Context) {
	beat := time.NewTicker(20 * time.Second)
	defer beat.Stop()
	s.beat()
	next := time.After(s.gap())
	restock := time.NewTicker(30 * time.Minute)
	defer restock.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-beat.C:
			s.beat()
		case <-restock.C:
			if !s.isPaused() {
				s.locked(func() error { return s.g.restock(time.Now(), false) })
			}
		case <-next:
			if !s.isPaused() {
				s.locked(s.one)
			}
			next = time.After(s.gap())
		}
	}
}

// gap is the wait before the next sale: the shop's volume shaped by the hour.
func (s *Sim) gap() time.Duration {
	h := time.Now().Hour()
	f := 0.7
	switch {
	case h >= 11 && h < 14, h >= 17 && h < 20:
		f = 1.6
	case h < 8 || h >= 21:
		f = 0.4
	}
	return time.Duration(float64(s.tick) / f * (0.5 + s.g.r.Float64()))
}

// locked runs one step; the time machine isn't used live, so its marks are dropped.
func (s *Sim) locked(f func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.g.tm.anchors = s.g.tm.anchors[:0]
	if err := f(); err != nil {
		log.Printf("demo: the shop's own trade: %v", err)
	}
}

// one is a customer: usually a plain sale, sometimes a plugin's kind of visit.
func (s *Sim) one() error {
	g := s.g
	if g.r.IntN(6) == 0 && len(g.s.p.Plugins) > 0 {
		if err := s.pluginVisit(g.s.p.Plugins[g.r.IntN(len(g.s.p.Plugins))]); err == nil {
			return nil
		}
	}
	return g.sale(g.cashier())
}

func (s *Sim) pluginVisit(key string) error {
	g := s.g
	now := time.Now()
	day := localDay(now)
	h := now.Sub(day).Hours()
	switch key {
	case "repairs":
		return g.repairIn(day, h)
	case "recharge":
		d, err := g.rechargeData()
		if err != nil {
			return err
		}
		return g.rechargeSale(d)
	case "documents":
		return documentsDaily(g, day, h+0.01)
	case "combos":
		return combosDaily(g, day, h+0.01)
	case "pharmacy":
		return pharmacyDaily(g, day, h+0.01)
	case "requests":
		return requestsDaily(g, day, h+0.01)
	}
	return fmt.Errorf("no visit for %s", key)
}

// beat tells Who's on now each staff member is at the till.
func (s *Sim) beat() {
	for _, c := range s.staff {
		cl, err := s.g.tillOf(c)
		if err != nil {
			continue
		}
		_ = cl.Do("POST", "/api/v1/presence", map[string]any{"path": "/till", "idleSec": s.g.r.IntN(40),
			"cartItems": float64(s.g.r.IntN(4)), "cartTotal": fmt.Sprint(s.g.r.IntN(30) * 50), "note": ""}, nil)
	}
}

// CheatInfo is one move in the chrome's cheat menu.
type CheatInfo struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Group string `json:"group"` // Till | Shop | Plugins | Looks | Demo
}

// CheatNames lists the moves for shop p (plugin moves only for its plugins).
func CheatNames(p shops.Profile) []CheatInfo {
	out := []CheatInfo{
		{"open-till", "Open tills for the shown logins", "Till"},
		{"busy", "Make it busy (20 sales now)", "Till"},
		{"closing", "Jump to closing time (fill the tills)", "Till"},
		{"low-stock", "Push a few products to low stock", "Shop"},
		{"credit-due", "A credit customer over their limit", "Shop"},
		{"delivery", "A supplier delivery arrives", "Shop"},
		{"print-samples", "Print a test receipt and label", "Shop"},
		{"cycle-palette", "Next palette", "Looks"},
		{"cycle-receipt", "Next receipt style", "Looks"},
		{"reset", "Reset this shop", "Demo"},
	}
	moves := map[string]CheatInfo{
		"repairs":   {"plugin:repairs", "A repair job comes in", "Plugins"},
		"recharge":  {"plugin:recharge", "A reload is sold", "Plugins"},
		"documents": {"plugin:documents", "A print & copy job", "Plugins"},
		"combos":    {"plugin:combos", "A combo is sold", "Plugins"},
		"pharmacy":  {"plugin:pharmacy", "A prescription is dispensed", "Plugins"},
		"requests":  {"plugin:requests", "A customer asks for something", "Plugins"},
	}
	for _, k := range p.Plugins {
		if m, ok := moves[k]; ok {
			out = append(out, m)
		}
	}
	return out
}

// Cheat performs one move from the menu.
func (s *Sim) Cheat(name string) error {
	var err error
	s.locked(func() error { err = s.cheat(name); return nil })
	return err
}

var palettes = []string{"studio", "night", "bazaar", "lagoon", "catppuccin", "gruvbox", "nord", "dracula", "tokyonight", "rosepine",
	"solarized", "everforest", "kanagawa", "synthwave", "monokai", "shadesofpurple", "cobalt2", "ayu", "mango", "pop", "aurora", "neumorph"}
var receiptStyles = []string{"classic", "bold", "minimal", "ticket", "poster", "grid", "stamp", "elegant", "cards", "sections"}

func (s *Sim) cheat(name string) error {
	g := s.g
	if k, ok := strings.CutPrefix(name, "plugin:"); ok {
		if !g.on(k) {
			return fmt.Errorf("%s isn't in this shop", k)
		}
		return s.pluginVisit(k)
	}
	switch name {
	case "open-till":
		return s.openTills()
	case "busy", "closing":
		n := 20
		if name == "closing" {
			n = g.s.p.Volume / 2
		}
		for i := 0; i < n; i++ {
			if err := g.sale(g.cashier()); err != nil {
				return err
			}
		}
		return nil
	case "low-stock":
		return s.lowStock()
	case "credit-due":
		return s.creditDue()
	case "delivery":
		for id := range g.stock {
			g.stock[id] = 0 // so every product is ordered
		}
		err := g.restock(time.Now(), false)
		_ = g.loadStock()
		return err
	case "print-samples":
		if err := g.admin.Do("POST", "/api/v1/admin/settings/printer/test", nil, nil); err != nil {
			return err
		}
		return g.admin.Do("POST", "/api/v1/till/labels/text", map[string]any{"lines": []string{g.s.p.Name, "Demo label", "Rs 1,250.00"}, "count": 1, "size": ""}, nil)
	case "cycle-palette", "cycle-receipt":
		return s.cycleLook(name == "cycle-palette")
	}
	return fmt.Errorf("no such move: %s", name)
}
