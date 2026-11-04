// Package printing is the transport layer shared by the receipt (ESC/POS) and
// label (TSPL) renderers. It sends raw bytes to a printer and discovers the
// printers available on the host so the admin can map "which printer for which"
// from the Settings UI.
//
// A print target is one of:
//   - "" (empty)            — the operating system's default printer.
//   - a printer/queue name  — an OS-installed printer (CUPS queue on Linux,
//     a Windows spooler printer on Windows). Discovered via Queues().
//   - "machine://3/XP80"    — a queue on a cashier PC, through its print helper
//   - "tcp://host:9100"     — a network printer addressed directly over a raw
//     socket, bypassing the OS spooler entirely (works the same on every OS).
//
// The OS-specific bits (spooler send + printer discovery) live in
// printing_unix.go / printing_windows.go behind build tags; this file holds the
// platform-independent dispatch and the network-socket path.
package printing

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Raw sends bytes to the print target unchanged. See the package doc for the
// accepted target formats. The bytes MUST reach the printer untouched (no driver
// rendering), so for OS-installed printers the queue must be a raw/passthrough
// one — otherwise the ESC/POS or TSPL stream is rasterized and prints garbage.
func Raw(ctx context.Context, target string, data []byte) error {
	if strings.HasPrefix(target, "machine://") {
		send := machineSender.Load()
		if send == nil {
			return errors.New("printing through other machines isn't available")
		}
		return (*send)(ctx, target, data)
	}
	if host, ok := tcpTarget(target); ok {
		return rawTCP(ctx, host, data)
	}
	return rawSpool(ctx, target, data)
}

// noSpool keeps tests off this PC's real printers: Go tests by themselves, the e2e server
// through its environment. A print with no printer set would go to the system default.
func noSpool() bool { return testing.Testing() || os.Getenv("KAROTS_NO_SPOOL") != "" }

// machineSender delivers "machine://…" targets: a printer on a cashier PC.
var machineSender atomic.Pointer[func(context.Context, string, []byte) error]

// SetMachineSender wires in that delivery (printing stays free of the database).
func SetMachineSender(send func(context.Context, string, []byte) error) {
	machineSender.Store(&send)
}

// Queues returns the printers installed on this host, used to populate the
// printer dropdowns in Settings. A discovery failure returns an empty list
// (never an error) so the Settings page still renders. Network ("tcp://…")
// printers are not OS-installed and so are entered by hand, not listed here.
func Queues(ctx context.Context) []string {
	return osQueues(ctx)
}

// tcpTarget reports whether target names a raw network printer and returns its
// host:port (defaulting the port to 9100, the standard raw/JetDirect port).
func tcpTarget(target string) (string, bool) {
	t := strings.TrimSpace(target)
	if !strings.HasPrefix(t, "tcp://") {
		return "", false
	}
	hostport := strings.TrimPrefix(t, "tcp://")
	if hostport == "" {
		return "", false
	}
	if _, _, err := net.SplitHostPort(hostport); err != nil {
		hostport = net.JoinHostPort(hostport, "9100")
	}
	return hostport, true
}

// rawTCP opens a raw TCP connection to a network printer and writes the bytes.
// Platform-independent — many thermal/label printers listen on port 9100.
func rawTCP(ctx context.Context, hostport string, data []byte) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", hostport)
	if err != nil {
		return fmt.Errorf("connect to network printer %s: %w", hostport, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("send to network printer %s: %w", hostport, err)
	}
	return nil
}
