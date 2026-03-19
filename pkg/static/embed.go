package static

import _ "embed"

// DashboardHTML contains the embedded dashboard page.
//
//go:embed html/dashboard.html
var DashboardHTML string
