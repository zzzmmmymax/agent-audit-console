package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/api"
	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/auth"
	runtimeconfig "github.com/agent-audit-console/agent-audit-console/internal/config"
)

func main() {
	address := flag.String("addr", "", "listen address (default 127.0.0.1:8777)")
	dataDir := flag.String("data-dir", "", "audit data directory")
	configFile := flag.String("config", "", "YAML config file")
	accessFile := flag.String("access-file", "", "YAML access policy; required for non-loopback binding")
	flag.Parse()
	resolved, err := runtimeconfig.Load(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	if *dataDir != "" {
		resolved, err = resolved.WithDataDir(*dataDir)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *address != "" {
		resolved.ListenAddress = *address
	}
	if *accessFile != "" {
		resolved.AccessFile = *accessFile
	}
	service, err := audit.OpenWithConfig(resolved)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()
	var access *auth.Manager
	if _, statErr := os.Stat(resolved.AccessFile); statErr == nil {
		access, err = auth.Load(resolved.AccessFile)
		if err != nil {
			log.Fatal(err)
		}
	} else if !os.IsNotExist(statErr) {
		log.Fatal(statErr)
	}
	if !isLoopbackAddress(resolved.ListenAddress) && access == nil {
		log.Fatal("refusing non-loopback listen address without --access-file or AGENT_AUDIT_ACCESS_FILE")
	}
	server := &http.Server{Addr: resolved.ListenAddress, Handler: api.NewWithPolicy(service.Store, service.Policy, access, service.Ready), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("Agent Audit Console: http://%s", resolved.ListenAddress)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
