package shops

// people gives a shop its shown logins (owner, manager, cashier) and three staff
// for the alive simulator. Phones are unique per shop database, so they repeat
// across shops; PINs are easy to type in front of a customer.
func people(owner, manager, cashier string, staff ...string) (shown, sim []Login) {
	shown = []Login{
		{Role: "admin", Name: owner, Phone: "0771000001", PIN: "1111"},
		{Role: "manager", Name: manager, Phone: "0771000002", PIN: "2222"},
		{Role: "cashier", Name: cashier, Phone: "0771000003", PIN: "3333"},
	}
	for i, n := range staff {
		sim = append(sim, Login{Role: "cashier", Name: n, Phone: "077200000" + string(rune('1'+i)), PIN: "4444"})
	}
	return shown, sim
}

// BuiltIn is the five shops the owner demos (user-approved, 2026-10-27).
func BuiltIn() []Profile {
	mk := func(p Profile, owner, manager, cashier string, staff ...string) Profile {
		p.Logins, p.Staff = people(owner, manager, cashier, staff...)
		return p
	}
	return []Profile{
		mk(Profile{
			Key: "grocery", Name: "Sunrise Mini Mart", Blurb: "Grocery / mini-mart — core only: packs, smart low stock, credit.",
			Palette: "studio", ReceiptStyle: "classic", Catalogue: "grocery", Volume: 140,
			LabelStyles: map[string]string{"price": "pricetag"},
		}, "Nimal Perera", "Sanduni Silva", "Kasun Fernando", "Dilani", "Ruwan"),
		mk(Profile{
			Key: "pharmacy", Name: "CityCare Pharmacy", Blurb: "Prescriptions, dose labels, batches and expiry, alternatives.",
			Plugins: []string{"pharmacy", "alternatives", "productplus", "requests"},
			Palette: "lagoon", ReceiptStyle: "minimal", Catalogue: "pharmacy", Volume: 90,
			LabelStyles: map[string]string{"price": "minimal", "pharmacy.dose": "clear"},
		}, "Dr. Fathima Rizvi", "Tharindu Jayasuriya", "Ishara Bandara", "Malsha", "Fazil"),
		mk(Profile{
			Key: "phone", Name: "Galaxy Mobile & Repairs", Blurb: "Repair jobs, reload float and commission, combos, invoices.",
			Plugins: []string{"repairs", "recharge", "combos", "documents"},
			Palette: "synthwave", ReceiptStyle: "ticket", Catalogue: "phone", Volume: 70,
			LabelStyles: map[string]string{"price": "qrtag", "repairs.ticket": "job"},
		}, "Ashan Wickramasinghe", "Rizwan Hameed", "Chamodi Herath", "Sahan", "Nadeesha"),
		mk(Profile{
			Key: "hardware", Name: "Lanka Build Hardware", Blurb: "Building supplies — quotes and invoices, clearance, customer requests.",
			Plugins: []string{"clearance", "productplus", "documents", "requests"},
			Palette: "bazaar", ReceiptStyle: "grid", Catalogue: "hardware", Volume: 60,
			LabelStyles: map[string]string{"price": "shelf"},
		}, "Sunil Rajapaksha", "Priyantha Kumara", "Hasini Dias", "Lahiru", "Kumari"),
		mk(Profile{
			Key: "everything", Name: "Karots Super Store", Blurb: "Every plugin at once, for the customer who wants to see it all.",
			Plugins: []string{"aicatalog", "alternatives", "clearance", "combos", "documents", "pharmacy", "productplus", "recharge", "repairs", "requests"},
			Palette: "aurora", ReceiptStyle: "cards", Catalogue: "mixed", Volume: 160,
			LabelStyles: map[string]string{"price": "dual", "pharmacy.dose": "boxed", "repairs.ticket": "job"},
		}, "Mohamed Imran", "Gayani Ekanayake", "Pradeep Senanayake", "Shehan", "Amaya", "Kavindu"),
	}
}

// Custom is a make-your-own shop: the picked plugins and looks over a mixed
// catalogue. Thrown away with everything else when the demo closes.
func Custom(plugins []string, palette, receiptStyle string) Profile {
	p := Profile{
		Key: "custom", Name: "My Shop", Blurb: "Your own mix.",
		Plugins: append([]string{}, plugins...), Palette: palette, ReceiptStyle: receiptStyle,
		Catalogue: "mixed", Volume: 100, LabelStyles: map[string]string{"price": "pricetag"},
	}
	if p.Palette == "" {
		p.Palette = "studio"
	}
	if p.ReceiptStyle == "" {
		p.ReceiptStyle = "classic"
	}
	p.Logins, p.Staff = people("Shop Owner", "Shop Manager", "Shop Cashier", "Dilani", "Ruwan")
	return p
}
