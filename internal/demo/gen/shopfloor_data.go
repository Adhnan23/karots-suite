package gen

// The shop floor beyond the shelf list: what a shop opens from bulk, sells by
// the pack, does as a service, and covers by warranty — per catalogue, so each
// shop works the way that kind of shop really does.

// bagLine is a bulk product the shop opens into what it sells loose. loss is
// how much a weighed bag can come up short (rice spills, onions dry out).
type bagLine struct {
	from, to string
	ratio    float64
	loss     float64
	note     string
}

// packLine sells a product by more than one at a time, a little cheaper.
type packLine struct {
	product, name string
	size          int
	price         string
}

// part is a stock item a service uses: per sale, or one item serving yield sales.
type part struct {
	product string
	amount  string
	yield   bool
}

type serviceLine struct {
	name, category, price string
	perWeek               float64
	parts                 []part
	costs                 [][2]string // label, cost per sale — not stock
}

// warrantyLine: sold with a serial number and the maker's warranty.
type warrantyLine struct {
	product string
	months  int
	imei    bool // a phone: a 15-digit IMEI rather than a maker's serial
	faults  []string
}

type floor struct {
	bags     []bagLine
	packs    []packLine
	services []serviceLine
	warranty []warrantyLine
	menu     map[string]string // till menu groups: category → emoji (in this order of rows)
	order    []string          // menu groups, busiest first
}

var floors = map[string]floor{
	"grocery": {
		bags: []bagLine{
			{"Samba Rice 25kg Bag", "Samba Rice 1kg", 25, 0.02, "Opened for the rice bin"},
			{"Red Raw Rice 25kg Bag", "Red Raw Rice 1kg", 25, 0.02, "Opened for the rice bin"},
			{"Mysore Dhal 25kg Bag", "Mysore Dhal 1kg", 25, 0.015, ""},
			{"White Sugar 50kg Bag", "White Sugar 1kg", 50, 0.01, "Weighed into 1kg bags"},
			{"Onions 10kg Net Bag", "Onions 1kg", 10, 0.06, "Some dried out"},
		},
		packs: []packLine{
			{"Eggs (each)", "Tray (30)", 30, "1590"},
			{"Yoghurt 80g", "Pack of 6", 6, "450"},
			{"Coca-Cola 1.5L", "Case (12)", 12, "4500"},
			{"Mineral Water 1.5L", "Pack (12)", 12, "1650"},
			{"Sunlight Soap 110g", "Bundle (4)", 4, "560"},
			{"Chocolate Bar 45g", "Box (24)", 24, "4900"},
		},
		services: []serviceLine{
			{"Plain Tea (cup)", "Services", "60", 45,
				[]part{{"Ceylon Tea 200g", "90", true}, {"White Sugar 1kg", "0.012", false}}, [][2]string{{"Gas", "4"}}},
			{"Milk Tea (cup)", "Services", "100", 25,
				[]part{{"Ceylon Tea 200g", "90", true}, {"White Sugar 1kg", "0.012", false}, {"Anchor Milk Powder 400g", "25", true}}, [][2]string{{"Gas", "4"}}},
			{"Home Delivery (within 3km)", "Services", "250", 9, nil, [][2]string{{"Rider & fuel", "180"}}},
			{"Gift Hamper Packing", "Services", "350", 1.5, nil, [][2]string{{"Basket, wrap & ribbon", "190"}}},
		},
		order: []string{"Rice & Grains", "Vegetables", "Dairy", "Bakery", "Groceries", "Beverages", "Spices", "Tea & Coffee", "Snacks", "Household", "Services"},
		menu: map[string]string{"Rice & Grains": "🍚", "Vegetables": "🧅", "Dairy": "🥛", "Bakery": "🍞", "Groceries": "🛒", "Beverages": "🥤",
			"Spices": "🌶️", "Tea & Coffee": "☕", "Snacks": "🍫", "Household": "🧼", "Services": "🛎️"},
	},
	"pharmacy": {
		packs: []packLine{
			{"Paracetamol 500mg (card of 10)", "Box (10 cards)", 10, "280"},
			{"Metformin 500mg (card of 10)", "Box (10 cards)", 10, "650"},
			{"Amlodipine 5mg (card of 10)", "Box (10 cards)", 10, "600"},
			{"Losartan 50mg (card of 10)", "Box (10 cards)", 10, "1100"},
			{"Atorvastatin 10mg (card of 10)", "Box (10 cards)", 10, "1300"},
			{"Cetirizine 10mg (card of 10)", "Box (10 cards)", 10, "550"},
			{"ORS Sachet", "Box (25)", 25, "1150"},
		},
		services: []serviceLine{
			{"Blood Pressure Check", "Services", "150", 22, nil, [][2]string{{"Cuff & battery wear", "8"}}},
			{"Blood Sugar Test", "Services", "350", 14,
				[]part{{"Glucometer Strips (25s)", "25", true}}, [][2]string{{"Lancet & swab", "25"}}},
			{"Wound Dressing", "Services", "450", 4,
				[]part{{"Bandage Roll 7.5cm", "0.5", false}, {"Antiseptic Cream 20g", "8", true}, {"Plasters (100s)", "40", true}}, nil},
			{"Nebulising", "Services", "500", 2, nil, [][2]string{{"Mask & saline", "120"}}},
		},
		warranty: []warrantyLine{
			{"BP Monitor", 24, false, []string{"Wrong readings", "Cuff leaks air", "Screen blank"}},
		},
		order: []string{"Pain & Fever", "Cold & Allergy", "Stomach", "Heart & BP", "Diabetes", "Vitamins", "First Aid", "Mother & Baby", "Skin Care", "Services"},
		menu: map[string]string{"Pain & Fever": "🤒", "Cold & Allergy": "🤧", "Stomach": "🫗", "Heart & BP": "❤️", "Diabetes": "🩸",
			"Vitamins": "🍊", "First Aid": "🩹", "Mother & Baby": "🍼", "Skin Care": "🧴", "Services": "🩺"},
	},
	"phone": {
		packs: []packLine{
			{"Tempered Glass (universal)", "Box (20)", 20, "4800"},
			{"USB-C Cable 1m", "Pack (5)", 5, "2600"},
			{"Silicone Back Cover", "Pack (10)", 10, "5000"},
		},
		services: []serviceLine{
			{"Tempered Glass Fitted", "Services", "600", 18, []part{{"Tempered Glass (universal)", "1", false}}, nil},
			{"Software Flash / Update", "Services", "1500", 3, nil, [][2]string{{"Box credits", "150"}}},
			{"Data Transfer (old → new phone)", "Services", "500", 4, nil, nil},
			{"Phone Unlock (pattern / FRP)", "Services", "1200", 1.5, nil, [][2]string{{"Box credits", "200"}}},
		},
		warranty: []warrantyLine{
			{"Samsung Galaxy A15 128GB", 12, true, []string{"Not charging", "Speaker not working", "Restarts by itself", "Touch not responding"}},
			{"Redmi 13C 128GB", 12, true, []string{"Not charging", "Camera blurry", "Restarts by itself", "SIM not detected"}},
			{"Infinix Hot 40i 128GB", 12, true, []string{"Not charging", "Hangs often", "Battery drains fast"}},
			{"Nokia 105 (feature phone)", 6, true, []string{"Keypad not working", "No signal"}},
			{"TWS Earbuds", 6, false, []string{"One side not working", "Case not charging", "Won't pair"}},
			{"Power Bank 10000mAh", 6, false, []string{"Not charging", "Drains overnight"}},
			{"Power Bank 20000mAh", 6, false, []string{"Not charging", "One port dead"}},
			{"Smart Watch Basic", 6, false, []string{"Strap broke at pin", "Won't sync", "Screen dead"}},
			{"Bluetooth Speaker Mini", 6, false, []string{"Crackling sound", "Won't charge"}},
		},
		order: []string{"Cases & Glass", "Cables", "Chargers", "Audio", "SIMs", "Power", "Storage", "Phones", "Accessories", "Services"},
		menu: map[string]string{"Cases & Glass": "🛡️", "Cables": "🔗", "Chargers": "🔌", "Audio": "🎧", "SIMs": "📶", "Power": "🔋",
			"Storage": "💾", "Phones": "📱", "Accessories": "⌚", "Services": "🛠️"},
	},
	"hardware": {
		bags: []bagLine{
			{"Wire 1.5mm (100m)", "Wire 1.5mm (per m)", 100, 0, "New coil on the reel"},
			{"Wire 2.5mm (100m)", "Wire 2.5mm (per m)", 100, 0, "New coil on the reel"},
			{"Wire Nails 2in 25kg Box", "Wire Nails 2in (1kg)", 25, 0.008, ""},
			{"Wire Nails 3in 25kg Box", "Wire Nails 3in (1kg)", 25, 0.008, ""},
			{"Binding Wire 25kg Coil", "Binding Wire 1kg", 25, 0.01, ""},
		},
		packs: []packLine{
			{"PVC Elbow 1/2in", "Bag (50)", 50, "2400"},
			{"PVC Tee 1/2in", "Bag (50)", 50, "3000"},
			{"LED Bulb 9W", "Box (10)", 10, "3900"},
			{"Switch Single Gang", "Box (10)", 10, "2950"},
			{"Insulation Tape", "Pack (10)", 10, "900"},
			{"Steel Rod 10mm (12m)", "Bundle (10)", 10, "32500"},
		},
		services: []serviceLine{
			{"Pipe Threading (per joint)", "Services", "150", 10, nil, [][2]string{{"Die wear", "12"}}},
			{"Paint Colour Tinting", "Services", "300", 4, nil, nil},
			{"Tractor Delivery (in town)", "Services", "1800", 5, nil, [][2]string{{"Driver & diesel", "1150"}}},
			{"Glass Cutting (per cut)", "Services", "100", 3, nil, nil},
		},
		warranty: []warrantyLine{
			{"Electric Drill 13mm", 12, false, []string{"Motor burnt", "Chuck slipping", "Switch faulty"}},
			{"Angle Grinder 4in", 12, false, []string{"Sparking at brushes", "Gearbox noise", "Switch faulty"}},
			{"Water Pump 0.5HP", 24, false, []string{"Not lifting water", "Seal leaking", "Motor hums, won't start"}},
		},
		order: []string{"Cement & Sand", "Fasteners", "Plumbing", "Electrical", "Paint", "Steel", "Roofing", "Tools", "Garden", "Services"},
		menu: map[string]string{"Cement & Sand": "🧱", "Fasteners": "🔩", "Plumbing": "🚰", "Electrical": "💡", "Paint": "🎨",
			"Steel": "🏗️", "Roofing": "🏠", "Tools": "🔨", "Garden": "🌱", "Services": "🛎️"},
	},
}

// floorFor is a catalogue's floor; the mixed store has every one.
func floorFor(catalogue string) floor {
	if f, ok := floors[catalogue]; ok {
		return f
	}
	var all floor
	all.menu = map[string]string{}
	for _, k := range []string{"grocery", "pharmacy", "phone", "hardware"} {
		f := floors[k]
		all.bags = append(all.bags, f.bags...)
		all.packs = append(all.packs, f.packs...)
		all.services = append(all.services, f.services...)
		all.warranty = append(all.warranty, f.warranty...)
		for _, c := range f.order {
			if _, ok := all.menu[c]; !ok {
				all.order = append(all.order, c)
				all.menu[c] = f.menu[c]
			}
		}
	}
	return all
}

// departments groups the mixed store's till menu (one card per kind of shop).
var departments = []struct{ name, emoji, catalogue string }{
	{"Grocery", "🛒", "grocery"}, {"Pharmacy", "💊", "pharmacy"}, {"Phones", "📱", "phone"}, {"Hardware", "🔨", "hardware"},
}
