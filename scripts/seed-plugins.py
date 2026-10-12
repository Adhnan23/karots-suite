"""Seed the demo shop (suite_accept on :3100, built with every plugin) with plugin data through the plugins' own APIs.
Usage: docker exec suite_db psql -U suite_user -d suite_accept -Atc "select json_agg(json_build_object('name',name,'id',id)) from products" > /tmp/p.json && python3 scripts/seed-plugins.py /tmp/p.json"""
import datetime, http.cookiejar, json, sys, urllib.request

B = "http://localhost:3100/api/v1"
op = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))


def call(method, path, body=None, ok_fail=False):
    req = urllib.request.Request(B + path, method=method, data=None if body is None else json.dumps(body).encode(),
                                 headers={"Content-Type": "application/json"})
    try:
        with op.open(req) as r:
            raw = r.read()
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        msg = e.read().decode()
        if ok_fail:
            print("  skip:", method, path, msg)
            return None
        sys.exit(f"FAIL {method} {path}: {e.code} {msg}")


call("POST", "/auth/login", {"phone": "0770000001", "pin": "1234"})  # Ayesha, owner
P = {p["name"]: p["id"] for p in json.loads(open(sys.argv[1]).read())}  # name -> id from the DB


def product(name, cat, cost, price, level):
    if name in P:
        return P[name]
    pid = call("POST", "/admin/products", {"name": name, "nameLocal": "", "barcode": "", "categoryId": cat, "unitId": 1,
               "costPrice": cost, "sellingPrice": price, "wholesalePrice": "", "taxRate": "", "reorderLevel": level,
               "trackSerial": False, "warrantyMonths": 0, "supplierId": 0, "repriceStock": False, "dupSourceName": ""})["id"]
    P[name] = pid
    return pid


# New products with no stock, so the till shows swaps and requests wait for stock.
product("Nadu Rice 1kg", 2, "190", "220", 20)
product("Fanta 1L", 4, "250", "310", 12)
product("Keeri Samba 1kg", 2, "260", "300", 10)

print("Product Plus")
def field(label, typ, scope=(), **kw):
    f = {"label": label, "type": typ, "defaultValue": "", "hint": "", "required": False, "searchable": False,
         "showAtTill": False, "printOnLabel": False, "showInIntake": False, "options": [], "scope": list(scope)}
    f.update(kw)
    return call("POST", "/x/productplus/fields", f)["id"]

brand = field("Brand", "text", searchable=True, showAtTill=True, printOnLabel=True, showInIntake=True,
              hint="Maker's name — the till search finds it")
storage = field("Storage", "select", options=["Dry shelf", "Cool place", "Fridge"], defaultValue="Dry shelf")
origin = field("Origin", "text", scope=[2], showAtTill=True, hint="Where the rice comes from")
shelf = field("Shelf life (months)", "number", scope=[1, 2, 3])
values = {
    brand: {"Sugar 1kg": "Pelwatte", "Tea Leaves 100g": "Dilmah", "Wheat Flour 1kg": "Prima", "Coconut Oil 750ml": "Marina",
            "Rice Samba 1kg": "Araliya", "Red Rice 1kg": "Nipuna", "Basmati 1kg": "India Gate", "Coca-Cola 1.5L": "Coca-Cola",
            "Sprite 1L": "Coca-Cola", "Fanta 1L": "Coca-Cola", "Mineral Water 1L": "Elephant House", "Milk Powder 400g": "Anchor",
            "Dish Soap 500ml": "Sunlight", "Laundry Powder 1kg": "Rinso", "Toothpaste 120g": "Signal", "Keeri Samba 1kg": "Araliya"},
    storage: {"Milk (ltr)": "Fridge", "Coconut Oil 750ml": "Cool place", "Milk Powder 400g": "Cool place"},
    origin: {"Rice Samba 1kg": "Polonnaruwa", "Red Rice 1kg": "Ampara", "Basmati 1kg": "India (imported)",
             "Nadu Rice 1kg": "Kurunegala", "Keeri Samba 1kg": "Polonnaruwa"},
    shelf: {"Sugar 1kg": "24", "Tea Leaves 100g": "24", "Wheat Flour 1kg": "6", "Coconut Oil 750ml": "12", "Rice Samba 1kg": "12",
            "Red Rice 1kg": "6", "Basmati 1kg": "18", "Milk Powder 400g": "18", "Mineral Water 1L": "12"},
}
call("PUT", "/x/productplus/values", [{"productId": P[n], "fieldId": f, "value": v} for f, m in values.items() for n, v in m.items()])

print("Combos")
today = datetime.date.today()
def combo(name, typ, val, items, starts="", ends=""):
    call("POST", "/x/combos", {"name": name, "discountType": typ, "discountValue": val, "startsOn": starts, "endsOn": ends,
         "items": [{"productId": P[n], "qty": q} for n, q in items]})

combo("Tea time set", "price", "990", [("Tea Leaves 100g", "1"), ("Milk Powder 400g", "1"), ("Sugar 1kg", "1")])
combo("Rice & oil saver", "percent", "10", [("Rice Samba 1kg", "5"), ("Coconut Oil 750ml", "1")])
combo("Cola party pack", "fixed", "100", [("Coca-Cola 1.5L", "2"), ("Sprite 1L", "2")],
      ends=(today + datetime.timedelta(days=14)).isoformat())
combo("Wash day", "percent", "5", [("Laundry Powder 1kg", "1"), ("Dish Soap 500ml", "1")],
      starts=(today + datetime.timedelta(days=7)).isoformat())

print("Alternatives")
def group(name):
    return call("POST", "/x/alternatives", {"name": name})

def member(tier, name):
    call("POST", f"/x/alternatives/tiers/{tier}/members", {"productId": P[name], "move": True})

rice = group("Rice 1kg")
blank = rice["tiers"][0]["id"]
call("PUT", f"/x/alternatives/tiers/{blank}", {"name": "Premium", "reorderLevel": 40})
for n in ("Basmati 1kg", "Keeri Samba 1kg"):
    member(blank, n)
every = call("POST", f"/x/alternatives/{rice['id']}/tiers", {"name": "Everyday", "reorderLevel": 1200})
for n in ("Rice Samba 1kg", "Red Rice 1kg", "Nadu Rice 1kg"):
    member(every["id"], n)
drinks = group("Soft drinks 1L+")
dt = drinks["tiers"][0]["id"]
call("PUT", f"/x/alternatives/tiers/{dt}", {"name": "", "reorderLevel": 300})
for n in ("Coca-Cola 1.5L", "Sprite 1L", "Fanta 1L"):
    member(dt, n)

print("Customer requests")
reqs = [("Kotmale yoghurt cups", "Nimal", "0771234567"), ("Oats 500g", "", ""), ("Brown sugar 1kg", "Sunethra", "0712223344"),
        ("Fanta 1L", "Kamal", "0763334455"), ("AA batteries", "", ""), ("Maliban cream crackers", "Ruwani", "0754445566")]
ids = {}
for note, name, phone in reqs:
    ids[note] = call("POST", "/x/requests", {"note": note, "name": name, "phone": phone})["id"]
call("POST", f"/x/requests/{ids['Brown sugar 1kg']}/asks", {"name": "Priyanka", "phone": "0779998877"})
call("POST", f"/x/requests/{ids['Oats 500g']}/asks", {"name": "", "phone": ""})
call("POST", f"/x/requests/{ids['Fanta 1L']}/link", {"productId": P["Fanta 1L"]})
call("POST", f"/x/requests/{ids['AA batteries']}/decline", {"reason": "Not a line we keep"}, ok_fail=True)

print("Clearance")
call("PUT", "/x/clearance/settings", {"staleDays": 21, "defaultPercent": "15", "minMarginPercent": "5", "expiryDays": 30, "deadMonths": 1})
page = call("GET", "/x/clearance")
print("  clearance page keys:", {k: (len(v) if isinstance(v, list) else v) for k, v in (page or {}).items() if k != "settings"})
print("done")
