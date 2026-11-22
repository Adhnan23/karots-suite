// Command demo is the Demo Room: the real suite as five believable, busy shops
// for showing a customer. The launcher (lock, picker, proxy, printers) runs each
// shop as a child process of this same binary (-child), with only that shop's
// plugins, on a throwaway database. Nothing survives a close.
package main

import (
	"flag"
	"os"
)

func main() {
	child := flag.Bool("child", false, "internal: run one shop (started by the launcher)")
	port := flag.Int("port", 3200, "launcher port (the next free one if taken)")
	host := flag.String("host", "127.0.0.1", "address to listen on: 0.0.0.0 inside a server's container, behind its reverse proxy")
	tpl := flag.String("templates", "", "build every built-in shop's year into this folder and exit (make demo)")
	flag.Parse()
	if *tpl != "" {
		os.Exit(runTemplates(*tpl))
	}
	if *child {
		os.Exit(runChild())
	}
	os.Exit(runLauncher(*host, *port))
}
