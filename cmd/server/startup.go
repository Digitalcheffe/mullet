package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Digitalcheffe/mullet/internal/db"
	plugindata "github.com/Digitalcheffe/mullet/internal/plugins/data"
)

// ASCII Shadow figlet rendering of "MULLET" -- printed once at the very
// start of main(), before anything else has a chance to fail, the same
// way projects like LinuxServer.io's container images open their own
// startup log with a banner before getting into real diagnostics.
const banner = `
███╗   ███╗██╗   ██╗██╗     ██╗     ███████╗████████╗
████╗ ████║██║   ██║██║     ██║     ██╔════╝╚══██╔══╝
██╔████╔██║██║   ██║██║     ██║     █████╗     ██║
██║╚██╔╝██║██║   ██║██║     ██║     ██╔══╝     ██║
██║ ╚═╝ ██║╚██████╔╝███████╗███████╗███████╗   ██║
╚═╝     ╚═╝ ╚═════╝ ╚══════╝╚══════╝╚══════╝   ╚═╝
`

const startupRule = "───────────────────────────────────────────────────"

// printBanner writes directly to stdout via fmt, not through the
// shared logger -- it's decoration for whoever's watching the terminal
// at boot (or the first lines of `docker logs`), not an operational
// log line someone greps for later, and printing it unconditionally
// this way means it still shows even if AUTH_DISABLED or a future
// logging misconfiguration were to affect the shared logger's output.
func printBanner() {
	fmt.Print(banner) // banner already ends in its own newline
	fmt.Println("  Self-hosted digital signage for the home")
	fmt.Println("  https://github.com/Digitalcheffe/mullet")
	fmt.Println(startupRule)
}

// logStartupLine reports one subsystem's outcome through the shared
// logger (so it lands in the configured log file too, not just
// stdout) -- prefixed distinctly from the framework's other log output
// (the scheduler's per-tick lines, the access log, etc.) so a `grep
// startup` on a log file pulls out exactly this boot-time summary.
func logStartupLine(format string, args ...any) {
	log.Printf("[startup] "+format, args...)
}

// logStartupSummary reports whether every subsystem main() depends on
// actually came up working, once, right before the HTTP server starts
// accepting connections -- the point of this (matching the request that
// added it) is that reading a fresh deployment's logs once should be
// enough to tell the database opened and migrated, logging is going
// where it's supposed to, an admin account and JWT secret exist,
// outgoing mail is (or deliberately isn't) configured, and the data
// plugins actually registered and have working instances -- not just a
// bare "listening" line with no way to tell whether anything upstream
// of it actually worked.
func logStartupSummary(sqldb *sql.DB, dbPath, logPath string) {
	logStartupLine("database ready: %s", dbPath)

	if logPath != "" {
		logStartupLine("logging: stdout + %s", logPath)
	} else {
		logStartupLine("logging: stdout only")
	}

	userCount, err := db.CountUsers(sqldb)
	if err != nil {
		logStartupLine("auth: could not count admin accounts: %v", err)
	} else {
		logStartupLine("auth: JWT secret ready, %d admin account(s)", userCount)
	}

	if smtpCfg, found, err := db.GetSMTPConfig(sqldb); err != nil {
		logStartupLine("smtp: could not load configuration: %v", err)
	} else if found {
		logStartupLine("smtp: configured (%s)", smtpCfg.Host)
	} else {
		logStartupLine("smtp: not configured -- password reset and email notifications stay off until set via Settings or SMTP_* env vars")
	}

	registered := len(plugindata.Default.List())
	statuses, err := db.ListPluginInstanceStatuses(sqldb)
	if err != nil {
		logStartupLine("data plugins: %d registered, could not count configured instances: %v", registered, err)
	} else {
		enabled := 0
		for _, s := range statuses {
			if s.Enabled {
				enabled++
			}
		}
		logStartupLine("data plugins: %d registered, %d/%d configured instances enabled", registered, enabled, len(statuses))
	}
}
