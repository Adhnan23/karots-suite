package gen

import (
	"context"
	"fmt"

	"karots-suite/internal/demo/shops"
	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/settings"
)

// shopSettings gives the shop its name, looks and the emulated printers. Every
// shop prints receipts and labels as Image (spec), without asking.
func shopSettings(ctx context.Context, d Deps, p shops.Profile) error {
	w, h := 50, 30
	if p.LabelStyles["price"] == "shelf" {
		w, h = 100, 50 // Shelf needs a label 70 mm or wider
	}
	_, err := d.DB.ExecContext(ctx, `UPDATE settings SET
		shop_name=$1, address=$2, phone=$3, receipt_width='80', ask_to_print=false,
		print_as='image', label_print_as='image',
		receipt_printer=$4, label_printer=$5, label_width_mm=$6, label_height_mm=$7
		WHERE id = 1`,
		p.Name, address(p), "011 "+p.Logins[0].Phone[3:6]+" "+p.Logins[0].Phone[6:],
		"tcp://"+d.ReceiptPrinter, "tcp://"+d.LabelPrinter, w, h)
	if err != nil {
		return fmt.Errorf("shop settings: %w", err)
	}
	return settings.NewRepository(d.DB).SetAppearance(ctx, p.Palette, p.ReceiptStyle, "", "", settings.LabelStyles(p.LabelStyles))
}

func address(p shops.Profile) string {
	switch p.Catalogue {
	case "pharmacy":
		return "42 Galle Road, Wellawatte, Colombo 06"
	case "phone":
		return "17 Main Street, Kandy"
	case "hardware":
		return "220 Negombo Road, Wattala"
	default:
		return "88 High Level Road, Maharagama"
	}
}

// people creates the shown logins and the simulator's staff, ready to sign in
// (no forced PIN change in a demo).
func people(ctx context.Context, d Deps, p shops.Profile) error {
	for _, l := range append(append([]shops.Login{}, p.Logins...), p.Staff...) {
		if _, err := d.Svc.Auth.CreateUser(ctx, auth.CreateUserInput{
			Name: l.Name, Phone: l.Phone, Role: l.Role, PIN: l.PIN,
			CanHandleSuppliers: "on", CanManageCredit: "on", CanSeeCost: "on", CanSeeLowStock: "on",
		}); err != nil {
			return fmt.Errorf("user %s: %w", l.Name, err)
		}
	}
	_, err := d.DB.ExecContext(ctx, `UPDATE users SET must_change_pin=false WHERE NOT is_system`)
	return err
}
