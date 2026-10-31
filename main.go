package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/KforG/p2pool-scanner-go/config"
	"github.com/KforG/p2pool-scanner-go/handler"
	"github.com/KforG/p2pool-scanner-go/logging"
	"github.com/KforG/p2pool-scanner-go/scanner"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to the config file")
	once := flag.Bool("once", false, "Scan once, write the nodes as JSON and exit instead of serving /nodes")
	out := flag.String("out", "-", "With -once: file to write the JSON to, or - for stdout")
	timeout := flag.Duration("timeout", 10*time.Minute, "With -once: stop scanning after this long and write what was found")
	flag.Parse()

	//init logging
	logFile, _ := os.OpenFile("debug.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if *once {
		// Keep stdout for the JSON.
		logging.SetLogWriter(io.MultiWriter(os.Stderr, logFile))
	} else {
		logging.SetLogFile(logFile)
	}

	logging.Infof("P2pool-Scannner-Go started up! \n")

	//init config
	err := config.ReadConfigFile(*configPath)
	if err != nil {
		panic(err) //We can't continue the program if the config can't be loaded
	}
	logging.Infof("Config loaded successfully\n")

	if *once {
		if err := runOnce(*out, *timeout); err != nil {
			logging.Errorf("%v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start scanning
	nodes := &scanner.NodeList{}
	go scanner.Scanner(nodes)

	// API endpoint(s)
	go handler.Router(nodes)

	// Hold up program for Go routines and exit gracefully
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	logging.Infof("Input detected, exiting application..\n")
}

// runOnce scans until done or until timeout, then writes the nodes found as
// JSON to out ("-" for stdout), sorted by address.
func runOnce(out string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	nodes := &scanner.NodeList{}
	scanner.ScanOnce(ctx, nodes)
	if ctx.Err() != nil {
		logging.Warnf("Scan stopped at the %v time limit, writing what it found\n", timeout)
	}

	found := nodes.Snapshot()
	sort.Slice(found, func(i, j int) bool { return found[i].IP < found[j].IP })
	data, err := json.MarshalIndent(found, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	logging.Infof("Found %d nodes\n", len(found))
	if out == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(out, data, 0644)
}
