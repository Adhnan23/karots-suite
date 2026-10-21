package receiptimg

import (
	"context"
	"io/fs"

	"karots-suite/internal/escpos"
	"karots-suite/internal/features/settings"
)

// SlipOptions renders the shop logo and the secondary (non-Latin) shop name to
// ESC/POS raster blocks so EVERY thermal slip — sale, return, warranty, credit,
// money receipt, a plugin slip — carries the same branding. Failures are non-fatal:
// the receipt still prints without that element. Callers pass the embedded static
// FS so a "/static/…" logo path resolves; a data: URI or http(s) logo needs no FS.
func SlipOptions(ctx context.Context, cfg *settings.Settings, staticFS fs.FS) escpos.Options {
	var opts escpos.Options
	if cfg == nil {
		return opts
	}
	dots := PrinterDots(cfg.ReceiptWidth)
	if src := cfg.LogoSrc(); src != "" {
		if img, err := LoadImage(ctx, src, staticFS); err == nil {
			opts.Logo = Logo(img, dots)
		}
	}
	if cfg.ShopNameSi != nil && *cfg.ShopNameSi != "" {
		opts.SubName = SubName(*cfg.ShopNameSi, dots, dots/14)
	}
	return opts
}

// Shop is the header and footer every slip carries, for a slip's React template
// (Image mode, ?data=1): the shop's own text (not ASCII'd — the template can draw
// Sinhala / Tamil) and the logo exactly as Classic prints it, its raster decoded
// to a PNG data URI ("" = none).
type Shop struct {
	Name      string `json:"name"`
	NameLocal string `json:"nameLocal"`
	Address   string `json:"address"`
	Phone     string `json:"phone"`
	VatNo     string `json:"vatNo"` // "" unless the shop is tax registered
	Footer    string `json:"footer"`
	Logo      string `json:"logo"`
}

func ShopFor(cfg *settings.Settings, opts escpos.Options) Shop {
	s := Shop{Name: cfg.ShopName, NameLocal: deref(cfg.ShopNameSi), Address: deref(cfg.Address), Phone: deref(cfg.Phone),
		Footer: deref(cfg.ReceiptFooter)}
	if cfg.TaxRegistered {
		s.VatNo = deref(cfg.TaxRegNo)
	}
	if len(opts.Logo) > 0 {
		for _, l := range escpos.Preview(opts.Logo) {
			if l.Image != "" {
				s.Logo = l.Image
				break
			}
		}
	}
	return s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
