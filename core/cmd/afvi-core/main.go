// Command afvi-core is the sidecar of the AFVI TOOLS GUI. It prints one JSON
// line on stdout when it is ready:
//
//	{"event":"ready","port":53122,"token":"...","version":"..."}
//
// and then serves the API on that loopback port until stdin closes (the GUI
// died or exited) or it receives SIGINT/SIGTERM.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"afvi/tools/core/internal/i18n"
	"afvi/tools/core/internal/machine"
	"afvi/tools/core/internal/server"
)

// version is set with -ldflags "-X main.version=...".
var version = "dev"

// devMachine implements "afvi-core dev-machine": it adds a machine whose nodes
// are local directories to the configuration, so copies can be tried on
// writable paths while the real machines are read-only.
func devMachine(args []string) {
	fs := flag.NewFlagSet("dev-machine", flag.ExitOnError)
	cfgPath := fs.String("config", "", "configuration file (default: <user config dir>/AFVI_TOOLS/config.json)")
	name := fs.String("name", "LOCAL TEST", "machine name")
	dir := fs.String("dir", "", "directory that will hold FM1, FM2 and BM (required)")
	fs.Parse(args)
	if *dir == "" {
		log.Fatal("dev-machine: -dir is required")
	}
	path := *cfgPath
	if path == "" {
		p, err := machine.DefaultPath()
		if err != nil {
			log.Fatal(err)
		}
		path = p
	}
	store, _, err := machine.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatal(err)
	}
	m := machine.LocalMachine(*name, abs)
	if err := m.CreateDirs(); err != nil {
		log.Fatal(err)
	}
	err = store.Update(i18n.For(i18n.Default), func(c *machine.Config) error {
		for i := range c.Machines {
			if strings.EqualFold(c.Machines[i].Name, m.Name) {
				c.Machines[i] = m
				return nil
			}
		}
		c.Machines = append(c.Machines, m)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("machine %q -> %s (config %s)\n", m.Name, abs, path)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "dev-machine" {
		devMachine(os.Args[2:])
		return
	}
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("afvi-core: ")
	var (
		cfgPath    = flag.String("config", "", "configuration file (default: <user config dir>/AFVI_TOOLS/config.json)")
		listen     = flag.String("listen", "127.0.0.1:0", "loopback address to listen on")
		watchStdin = flag.Bool("watch-stdin", false, "exit when stdin is closed (the parent process died)")
		allowWrite = flag.Bool("allow-network-write", false, "lift the dev-phase rule that network paths are read-only")
		showVer    = flag.Bool("version", false, "print the version and exit")
	)
	flag.Parse()
	if *showVer {
		fmt.Println(version)
		return
	}

	path := *cfgPath
	if path == "" {
		p, err := machine.DefaultPath()
		if err != nil {
			log.Fatal(err)
		}
		path = p
	}
	store, created, err := machine.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	if created {
		log.Printf("created %s with the default machine", path)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *watchStdin {
		go func() {
			io.Copy(io.Discard, os.Stdin)
			stop()
		}()
	}

	srv := server.New(version, store, *allowWrite)
	addr, wait, err := srv.Serve(ctx, *listen)
	if err != nil {
		log.Fatal(err)
	}
	_, ps, _ := net.SplitHostPort(addr.String())
	port, _ := strconv.Atoi(ps)
	ready, _ := json.Marshal(map[string]any{"event": "ready", "port": port, "token": srv.Token(), "version": version})
	fmt.Println(string(ready))
	if *allowWrite {
		log.Print("WARNING: network writes are enabled")
	}
	if err := wait(); err != nil {
		log.Fatal(err)
	}
}
