package gen

// Catalogue lines: category ; name ; local name ; unit ; cost ; price ; sold per week.
// Prices in LKR, late 2026.

var groceryLines = `
Rice & Grains;Samba Rice 1kg;සම්බා සහල් 1kg;kg;235;265;60
Rice & Grains;Samba Rice 25kg Bag;සම්බා සහල් 25kg;pcs;5700;6400;0.4
Rice & Grains;Red Raw Rice 1kg;රතු කැකුළු 1kg;kg;210;240;45
Rice & Grains;Red Raw Rice 25kg Bag;சிவப்பு பச்சை அரிசி 25kg;pcs;5100;5800;0.3
Rice & Grains;Keeri Samba 1kg;කීරි සම්බා 1kg;kg;290;330;30
Rice & Grains;Basmati Rice 1kg;பாஸ்மதி அரிசி 1kg;pkt;650;740;8
Rice & Grains;Mysore Dhal 1kg;පරිප්පු 1kg;kg;290;330;40
Rice & Grains;Mysore Dhal 25kg Bag;மைசூர் பருப்பு 25kg;pcs;6950;7900;0.2
Rice & Grains;Green Gram 500g;මුං ඇට;pkt;360;410;12
Rice & Grains;Chickpeas 500g;කඩල;pkt;310;360;10
Groceries;White Sugar 1kg;සීනි 1kg;kg;235;260;55
Groceries;White Sugar 50kg Bag;සීනි 50kg;pcs;11250;12600;0.2
Groceries;Brown Sugar 1kg;சிவப்பு சீனி 1kg;kg;300;345;10
Groceries;Wheat Flour 1kg;තිරිඟු පිටි;kg;200;230;35
Groceries;Rice Flour 1kg;සහල් පිටි;pkt;260;300;12
Groceries;Salt 400g;ලුණු;pkt;70;90;20
Groceries;Coconut Oil 1L;පොල් තෙල්;btl;690;780;25
Groceries;Soya Meat 90g;සෝයා මීට්;pkt;95;120;30
Groceries;Sprats 250g;හාල්මැස්සෝ;pkt;300;350;14
Groceries;Maldive Fish 100g;උම්බලකඩ;pkt;420;480;9
Groceries;Canned Mackerel 425g;ටින් මාළු;pcs;440;510;18
Spices;Chilli Powder 100g;මිරිස් කුඩු;pkt;180;220;20
Spices;Curry Powder 100g;තුනපහ;pkt;170;210;22
Spices;Turmeric 50g;කහ කුඩු;pkt;95;120;15
Spices;Cinnamon Sticks 50g;කුරුඳු;pkt;240;290;6
Tea & Coffee;Ceylon Tea 200g;තේ කොළ 200g;pkt;390;450;28
Tea & Coffee;Tea Bags 50s;தேயிலை பைகள் 50;box;520;600;10
Tea & Coffee;Nescafe Classic 50g;;pcs;780;890;6
Tea & Coffee;Milo 400g;;pkt;980;1120;12
Dairy;Anchor Milk Powder 400g;කිරිපිටි 400g;pkt;1090;1250;30
Dairy;Fresh Milk 1L;දියර කිරි;btl;420;480;20
Dairy;Curd 1L;මී කිරි;pcs;620;720;14
Dairy;Yoghurt 80g;යෝගට්;pcs;65;80;60
Dairy;Butter 200g;බටර්;pcs;820;940;6
Dairy;Eggs (each);බිත්තර;pcs;46;55;180
Bakery;Sandwich Bread 450g;පාන්;pcs;170;200;45
Bakery;Marie Biscuits 360g;මාරි බිස්කට්;pkt;300;350;25
Bakery;Cream Crackers 500g;கிறீம் கிறாக்கர் 500g;pkt;420;490;20
Bakery;Chocolate Puff 200g;;pkt;290;340;14
Beverages;Coca-Cola 1.5L;;btl;330;390;24
Beverages;Elephant House Ginger Beer 1L;;btl;290;340;18
Beverages;Mineral Water 1.5L;ජලය;btl;110;150;40
Beverages;Faluda 400ml;;btl;230;270;10
Beverages;Orange Juice 1L;தோடம்பழச் சாறு 1L;pcs;560;640;8
Household;Sunlight Soap 110g;සන්ලයිට්;pcs;125;150;30
Household;Washing Powder 1kg;சலவைத் தூள் 1kg;pkt;690;790;14
Household;Dishwash Liquid 500ml;பாத்திரம் கழுவும் திரவம் 500ml;btl;390;450;10
Household;Toilet Paper 4 rolls;கழிவறை காகிதம் 4;pkt;520;600;12
Household;Matches (10 boxes);ගිනිපෙට්ටි;pkt;95;120;20
Personal Care;Toothpaste 120g;දත් බෙහෙත්;pcs;290;340;16
Personal Care;Shampoo 180ml;ஷாம்பு 180ml;btl;560;650;8
Personal Care;Lux Soap 100g;;pcs;160;190;20
Snacks;Potato Chips 100g;உருளைக்கிழங்கு சிப்ஸ் 100g;pkt;290;350;22
Snacks;Chocolate Bar 45g;சொக்லேட் 45g;pcs;180;220;40
Snacks;Peanuts 100g;රටකජු;pkt;140;180;25
Vegetables;Onions 1kg;ලූනු;kg;290;340;30
Vegetables;Onions 10kg Net Bag;வெங்காயம் 10kg;pcs;2700;3150;0.5
Vegetables;Potatoes 1kg;අල;kg;320;370;28
Vegetables;Garlic 250g;සුදුළූණු;pkt;210;260;14
`

var pharmacyLines = `
Pain & Fever;Paracetamol 500mg (card of 10);පැරසිටමෝල්;pcs;18;30;120
Pain & Fever;Ibuprofen 400mg (card of 10);;pcs;55;80;30
Pain & Fever;Diclofenac Gel 30g;;pcs;390;480;8
Pain & Fever;Aspirin 75mg (card of 14);;pcs;60;90;20
Antibiotics;Amoxicillin 500mg (card of 10);;pcs;140;190;25
Antibiotics;Azithromycin 500mg (3s);;pcs;360;450;10
Antibiotics;Cephalexin 500mg (card of 10);;pcs;210;280;8
Antibiotics;Metronidazole 400mg (card of 10);;pcs;75;110;10
Diabetes;Metformin 500mg (card of 10);;pcs;45;70;60
Diabetes;Gliclazide 80mg (card of 10);;pcs;70;100;25
Diabetes;Glucometer Strips (25s);;box;2200;2650;4
Heart & BP;Losartan 50mg (card of 10);;pcs;80;120;45
Heart & BP;Amlodipine 5mg (card of 10);;pcs;40;65;50
Heart & BP;Atorvastatin 10mg (card of 10);;pcs;95;140;35
Heart & BP;Atenolol 50mg (card of 14);;pcs;55;85;15
Stomach;Omeprazole 20mg (card of 14);;pcs;85;130;40
Stomach;Antacid Syrup 200ml;;btl;390;470;12
Stomach;ORS Sachet;ජීවනී;pcs;35;50;40
Stomach;Domperidone 10mg (card of 10);;pcs;50;75;12
Cold & Allergy;Cetirizine 10mg (card of 10);;pcs;35;60;45
Cold & Allergy;Cough Syrup 100ml;කැස්ස බෙහෙත්;btl;420;520;20
Cold & Allergy;Chlorpheniramine 4mg (card of 10);;pcs;20;35;20
Cold & Allergy;Nasal Spray 15ml;;pcs;650;790;6
Cold & Allergy;Siddhalepa Balm 25g;සිද්ධාලේප;pcs;260;320;14
Vitamins;Vitamin C 500mg (card of 10);;pcs;95;140;30
Vitamins;Multivitamin (30s);;box;1250;1500;6
Vitamins;Calcium + D3 (card of 10);;pcs;180;240;14
Vitamins;Folic Acid 5mg (card of 10);;pcs;30;50;10
Vitamins;Iron Syrup 200ml;;btl;560;680;6
Skin Care;Sunscreen SPF50 50ml;;pcs;1450;1750;4
Skin Care;Calamine Lotion 100ml;;btl;290;360;6
Skin Care;Antiseptic Cream 20g;;pcs;220;280;10
First Aid;Bandage Roll 7.5cm;;pcs;110;150;12
First Aid;Plasters (100s);;box;480;580;6
First Aid;Surgical Spirit 100ml;;btl;180;230;8
First Aid;Face Masks (50s);;box;650;800;6
Mother & Baby;Baby Diapers M (30s);;pkt;2350;2750;10
Mother & Baby;Baby Wipes 80s;;pkt;520;620;10
Mother & Baby;Infant Formula 400g;;pkt;2900;3350;8
Devices;Digital Thermometer;;pcs;690;890;3
Devices;BP Monitor;;pcs;8500;10500;1
Controlled;Tramadol 50mg (card of 10);;pcs;120;180;0
Controlled;Diazepam 5mg (card of 10);;pcs;35;60;0
Controlled;Codeine Linctus 100ml;;btl;520;650;0
`

var phoneLines = `
Phones;Samsung Galaxy A15 128GB;;pcs;48500;54900;3
Phones;Redmi 13C 128GB;;pcs;33500;38900;4
Phones;Nokia 105 (feature phone);;pcs;5200;6500;5
Phones;Infinix Hot 40i 128GB;;pcs;31000;35900;2
Chargers;20W USB-C Fast Charger;;pcs;1450;2250;12
Chargers;Car Charger Dual USB;;pcs;650;1100;5
Chargers;Wireless Charging Pad;;pcs;1800;2900;3
Cables;USB-C Cable 1m;;pcs;290;600;25
Cables;Lightning Cable 1m;;pcs;390;750;12
Cables;Micro USB Cable 1m;;pcs;180;400;15
Audio;Wired Earphones;;pcs;350;750;14
Audio;TWS Earbuds;;pcs;2900;4500;6
Audio;Bluetooth Speaker Mini;;pcs;3200;4900;3
Cases & Glass;Tempered Glass (universal);;pcs;90;400;40
Cases & Glass;Silicone Back Cover;;pcs;180;650;30
Cases & Glass;Flip Cover;;pcs;450;1200;10
Power;Power Bank 10000mAh;;pcs;2950;4500;6
Power;Power Bank 20000mAh;;pcs;4600;6900;3
Storage;Memory Card 32GB;;pcs;1100;1750;6
Storage;Memory Card 64GB;;pcs;1650;2500;5
Storage;Pen Drive 32GB;;pcs;950;1600;5
SIMs;Dialog SIM;;pcs;100;200;8
SIMs;Mobitel SIM;;pcs;100;200;6
Spares;Phone Battery (universal);;pcs;1200;2400;3
Spares;Charging Port Flex;;pcs;450;1500;2
Accessories;Phone Holder (car);;pcs;690;1250;4
Accessories;Selfie Stick;;pcs;750;1350;2
Accessories;Smart Watch Basic;;pcs;4200;6500;2
`

var hardwareLines = `
Cement & Sand;Cement 50kg (Tokyo);සිමෙන්ති;pcs;2150;2450;45
Cement & Sand;Cement 50kg (INSEE);;pcs;2200;2500;30
Cement & Sand;Tile Adhesive 25kg;;pcs;1900;2350;10
Steel;Steel Rod 10mm (12m);;pcs;2900;3350;20
Steel;Steel Rod 12mm (12m);;pcs;4100;4700;12
Steel;Binding Wire 1kg;;kg;520;620;14
Steel;Binding Wire 25kg Coil;;pcs;12000;14000;0.1
Steel;GI Pipe 1/2in (6m);;pcs;3200;3700;6
Plumbing;PVC Pipe 1/2in (6m);;pcs;690;820;25
Plumbing;PVC Pipe 1in (6m);;pcs;1450;1700;12
Plumbing;PVC Elbow 1/2in;;pcs;35;55;80
Plumbing;PVC Tee 1/2in;;pcs;45;70;50
Plumbing;Ball Valve 1/2in;;pcs;390;520;12
Plumbing;Water Tap Chrome;;pcs;1450;1900;8
Plumbing;PVC Solvent 100ml;;btl;290;380;15
Plumbing;Water Pump 0.5HP;;pcs;14500;17900;0.5
Electrical;Wire 1.5mm (100m);;pcs;11500;13500;0.8
Electrical;Wire 1.5mm (per m);;m;115;150;40
Electrical;Wire 2.5mm (100m);;pcs;17500;20500;0.5
Electrical;Wire 2.5mm (per m);;m;175;240;25
Electrical;Switch Single Gang;;pcs;220;320;30
Electrical;Socket 13A;;pcs;390;520;20
Electrical;LED Bulb 9W;;pcs;290;420;40
Electrical;LED Tube 18W;;pcs;890;1250;10
Electrical;Insulation Tape;;pcs;65;100;30
Paint;Emulsion Paint 4L (white);;pcs;4900;5800;10
Paint;Enamel Paint 1L;;pcs;1650;1990;12
Paint;Wall Putty 5kg;;pcs;1350;1650;8
Paint;Paint Brush 2in;;pcs;180;260;20
Paint;Paint Roller 9in;;pcs;690;920;8
Paint;Thinner 1L;;btl;650;800;10
Tools;Hammer Claw 16oz;;pcs;1150;1500;4
Tools;Screwdriver Set;;pcs;1250;1700;4
Tools;Measuring Tape 5m;;pcs;490;690;6
Tools;Hacksaw Frame;;pcs;850;1150;3
Tools;Drill Bit Set;;pcs;1650;2200;3
Tools;Electric Drill 13mm;;pcs;8900;11500;1
Tools;Angle Grinder 4in;;pcs;7900;9900;0.8
Fasteners;Wire Nails 2in (1kg);ඇණ;kg;440;540;20
Fasteners;Wire Nails 2in 25kg Box;;pcs;10500;12500;0.2
Fasteners;Wire Nails 3in (1kg);;kg;440;540;14
Fasteners;Wire Nails 3in 25kg Box;;pcs;10500;12500;0.15
Fasteners;Screws 1in (100s);;box;290;390;15
Fasteners;Wall Plugs (100s);;box;190;280;12
Fasteners;Padlock 40mm;;pcs;690;950;6
Roofing;Roofing Sheet 8ft;;pcs;2650;3100;12
Roofing;Roofing Nails (1kg);;kg;690;850;6
Garden;Garden Hose 15m;;pcs;2400;3100;3
Garden;Mammoty;උදැල්ල;pcs;1650;2100;4
`

// suppliersFor: name ; contact ; phone ; credit days — one per category group, in order.
var supplierLines = map[string]string{
	"grocery": `Colombo Distributors (Pvt) Ltd;Ravi Jayasuriya;0114567890;30
Kandy Wholesale Traders;Anil Bandara;0812233445;14
Lanka Dairies Agency;Shanika Mendis;0119988776;7
Ceylon Beverages Agency;Fazal Rahman;0112345678;14`,
	"pharmacy": `State Pharmaceuticals Corp;Dr. Nalin Perera;0112320356;30
Hemas Pharmaceuticals;Iresha Gunawardena;0114766666;45
Emerchemie NB;Mahesh Silva;0112696220;30`,
	"phone": `Mobile Hub Distributors;Asif Naleem;0112440088;14
Pettah Accessories Wholesale;Kanthan Raj;0112333477;7
SIM & Reload Agency;Dinesh Karunaratne;0777123456;0`,
	"hardware": `Tokyo Cement Agency;Saman Kumara;0112555333;30
Lanka Steel Traders;Rohan de Silva;0112699699;30
S-Lon Distributors;Chathura Perera;0114799799;45
Electro Mart Wholesale;Imtiaz Ali;0112422422;14`,
}

var customerLines = `Kamala Stores;0775554433;Main Street, Maharagama;25000
Sunil Fernando;0712221110;12 Temple Road, Nugegoda;5000
Mrs. Wijesinghe;0718882211;45 Lake Drive, Boralesgamuwa;8000
Hotel Green Leaf;0112777888;Highlevel Road, Pannipitiya;50000
Rizwan Traders;0779090901;3 Market Lane, Kottawa;30000
Chamari Jayawardena;0761234321;;3000
Pradeep Construction;0773331100;Hokandara Road, Thalawathugoda;75000
Saman Perera;0714448899;;5000
Ayesha Farook;0756667788;8/2 Mosque Road, Dehiwala;6000
Lakmal Herath;0727776655;;4000`
