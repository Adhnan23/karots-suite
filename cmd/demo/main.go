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
	port := flag.Int("port", 3200, "launcher port on 127.0.0.1 (the next free one if taken)")
	flag.Parse()
	if *child {
		os.Exit(runChild())
	}
	os.Exit(runLauncher(*port))
}
