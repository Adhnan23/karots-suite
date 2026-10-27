package gen

import "time"

// today trades from opening to now with the tills left open.
func (g *gen) today(now time.Time) error {
	if err := g.tm.begin(g.ctx); err != nil {
		return err
	}
	day := localDay(now)
	if err := g.day(day, 0, now.Sub(day).Hours()-0.02); err != nil {
		return err
	}
	return g.tm.shift(g.ctx)
}
