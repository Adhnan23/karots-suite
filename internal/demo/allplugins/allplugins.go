// Package allplugins compiles every plugin into the Demo Room (the demo binary and
// its tests); each shop then keeps only its own with plugin.Only.
package allplugins

import (
	_ "karots-suite/plugins/aicatalog"
	_ "karots-suite/plugins/alternatives"
	_ "karots-suite/plugins/clearance"
	_ "karots-suite/plugins/combos"
	_ "karots-suite/plugins/documents"
	_ "karots-suite/plugins/pharmacy"
	_ "karots-suite/plugins/productplus"
	_ "karots-suite/plugins/recharge"
	_ "karots-suite/plugins/repairs"
	_ "karots-suite/plugins/requests"
)
