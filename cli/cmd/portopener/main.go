package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/AidyyJ/PortOpener/cli/internal/config"
	"github.com/AidyyJ/PortOpener/cli/internal/daemon"
	"github.com/AidyyJ/PortOpener/cli/internal/relayclient"
)

func main() {
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("portopener (dev) %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		return
	}

	switch args[0] {
	case "relay":
		runRelay(args[1:])
	case "http":
		runHTTP(args[1:])
	case "tcp":
		runTCP(args[1:])
	case "udp":
		runUDP(args[1:])
	case "start":
		runStart(args[1:])
	case "daemon":
		runDaemon(args[1:])
	case "init":
		runInit(args[1:])
	default:
		printUsage()
	}
}

func runRelay(args []string) {
	fs := flag.NewFlagSet("relay", flag.ExitOnError)
	url := fs.String("url", getenv("PORTOPENER_RELAY_URL", "ws://localhost/relay"), "relay websocket url")
	token := fs.String("token", getenv("PORTOPENER_RELAY_TOKEN", getenv("PORTOPENER_ADMIN_TOKEN", "")), "relay token")
	clientID := fs.String("client-id", "", "client id (uuid if empty)")
	heartbeat := fs.Duration("heartbeat", 10*time.Second, "heartbeat interval")
	fs.Parse(args)

	resolvedToken := resolveToken(*token)
	if strings.TrimSpace(resolvedToken) == "" {
		log.Fatal("relay token is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := relayclient.New(relayclient.Config{
		URL:             *url,
		Token:           resolvedToken,
		ClientID:        *clientID,
		HeartbeatPeriod: *heartbeat,
	})

	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("relay session failed: %v", err)
	}
}

func runHTTP(args []string) {
	fs := flag.NewFlagSet("http", flag.ExitOnError)
	url := fs.String("url", getenv("PORTOPENER_RELAY_URL", "ws://localhost/relay"), "relay websocket url")
	token := fs.String("token", getenv("PORTOPENER_RELAY_TOKEN", getenv("PORTOPENER_ADMIN_TOKEN", "")), "relay token")
	subdomain := fs.String("subdomain", "", "subdomain to register")
	allowlist := fs.String("allow", "", "comma-separated allowlist CIDRs")
	clientID := fs.String("client-id", "", "client id (uuid if empty)")
	local := fs.String("local", getenv("PORTOPENER_LOCAL_URL", "http://localhost:8081"), "local base url")
	localHost := fs.String("local-host", getenv("PORTOPENER_LOCAL_HOST", "localhost"), "local host for tunnel metadata")
	localPort := fs.Int("local-port", getenvInt("PORTOPENER_LOCAL_PORT", 8081), "local port for tunnel metadata")
	fs.Parse(args)

	resolvedToken := resolveToken(*token)
	if strings.TrimSpace(resolvedToken) == "" {
		log.Fatal("relay token is required")
	}
	if strings.TrimSpace(*subdomain) == "" {
		log.Fatal("subdomain is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := relayclient.New(relayclient.Config{
		URL:          *url,
		Token:        resolvedToken,
		ClientID:     *clientID,
		LocalBaseURL: *local,
		LocalHost:    *localHost,
		LocalPort:    *localPort,
	})

	var allowlistValues []string
	for _, value := range strings.Split(*allowlist, ",") {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			allowlistValues = append(allowlistValues, trimmed)
		}
	}

	if err := client.RegisterHTTP(ctx, *subdomain, allowlistValues); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("http tunnel registration failed: %v", err)
	}
}

func runTCP(args []string) {
	fs := flag.NewFlagSet("tcp", flag.ExitOnError)
	url := fs.String("url", getenv("PORTOPENER_RELAY_URL", "ws://localhost/relay"), "relay websocket url")
	token := fs.String("token", getenv("PORTOPENER_RELAY_TOKEN", getenv("PORTOPENER_ADMIN_TOKEN", "")), "relay token")
	externalPort := fs.Int("external-port", getenvInt("PORTOPENER_EXTERNAL_PORT", 0), "external TCP port to reserve")
	clientID := fs.String("client-id", "", "client id (uuid if empty)")
	localHost := fs.String("local-host", getenv("PORTOPENER_LOCAL_HOST", "localhost"), "local host to dial")
	localPort := fs.Int("local-port", getenvInt("PORTOPENER_LOCAL_PORT", 8081), "local port to dial")
	fs.Parse(args)

	resolvedToken := resolveToken(*token)
	if strings.TrimSpace(resolvedToken) == "" {
		log.Fatal("relay token is required")
	}
	if *externalPort == 0 {
		log.Fatal("external-port is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := relayclient.New(relayclient.Config{
		URL:       *url,
		Token:     resolvedToken,
		ClientID:  *clientID,
		LocalHost: *localHost,
		LocalPort: *localPort,
	})
	client.AddStreamHandler("tcp", client.HandleTCPStream)

	if err := client.RegisterTCP(ctx, *externalPort); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("tcp tunnel registration failed: %v", err)
	}
}

func runUDP(args []string) {
	fs := flag.NewFlagSet("udp", flag.ExitOnError)
	url := fs.String("url", getenv("PORTOPENER_RELAY_URL", "ws://localhost/relay"), "relay websocket url")
	token := fs.String("token", getenv("PORTOPENER_RELAY_TOKEN", getenv("PORTOPENER_ADMIN_TOKEN", "")), "relay token")
	externalPort := fs.Int("external-port", getenvInt("PORTOPENER_EXTERNAL_PORT", 0), "external UDP port to reserve")
	clientID := fs.String("client-id", "", "client id (uuid if empty)")
	localHost := fs.String("local-host", getenv("PORTOPENER_LOCAL_HOST", "localhost"), "local host to dial")
	localPort := fs.Int("local-port", getenvInt("PORTOPENER_LOCAL_PORT", 8081), "local port to dial")
	fs.Parse(args)

	resolvedToken := resolveToken(*token)
	if strings.TrimSpace(resolvedToken) == "" {
		log.Fatal("relay token is required")
	}
	if *externalPort == 0 {
		log.Fatal("external-port is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client := relayclient.New(relayclient.Config{
		URL:       *url,
		Token:     resolvedToken,
		ClientID:  *clientID,
		LocalHost: *localHost,
		LocalPort: *localPort,
	})
	client.AddStreamHandler("udp", client.HandleUDPStream)

	if err := client.RegisterUDP(ctx, *externalPort); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("udp tunnel registration failed: %v", err)
	}
}

func runStart(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	configPath := fs.String("config", getenv("PORTOPENER_CONFIG", ""), "config file path")
	fs.Parse(args)

	path := strings.TrimSpace(*configPath)
	if path == "" {
		path = config.DefaultPath()
	}
	loaded, err := config.Load(path)
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if strings.TrimSpace(loaded.Token) == "" {
		loaded.Token = resolveToken("")
	}
	if err := loaded.Validate(); err != nil {
		log.Fatalf("config invalid: %v", err)
	}
	printTunnelStatus(loaded)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup
	for _, tunnel := range loaded.Tunnels {
		t := tunnel
		wg.Add(1)
		go func() {
			defer wg.Done()
			runTunnelLoop(ctx, loaded, t)
		}()
	}

	<-ctx.Done()
	wg.Wait()
}

func runDaemon(args []string) {
	if len(args) == 0 {
		fmt.Println("daemon commands: start | stop | status")
		return
	}
	switch args[0] {
	case "start":
		runDaemonStart(args[1:])
	case "stop":
		runDaemonStop()
	case "status":
		runDaemonStatus(args[1:])
	default:
		fmt.Println("daemon commands: start | stop | status")
	}
}

func runDaemonStart(args []string) {
	fs := flag.NewFlagSet("daemon start", flag.ExitOnError)
	configPath := fs.String("config", getenv("PORTOPENER_CONFIG", ""), "config file path")
	fs.Parse(args)

	path := strings.TrimSpace(*configPath)
	if path == "" {
		path = config.DefaultPath()
	}
	loaded, err := config.Load(path)
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	if err := loaded.Validate(); err != nil {
		log.Fatalf("config invalid: %v", err)
	}

	pid, err := daemon.Start(path)
	if err != nil {
		log.Fatalf("daemon start failed: %v", err)
	}
	log.Printf("daemon started pid=%d", pid)
}

func runDaemonStop() {
	if err := daemon.Stop(); err != nil {
		log.Fatalf("daemon stop failed: %v", err)
	}
	log.Printf("daemon stopped")
}

func runDaemonStatus(args []string) {
	fs := flag.NewFlagSet("daemon status", flag.ExitOnError)
	configPath := fs.String("config", getenv("PORTOPENER_CONFIG", ""), "config file path")
	fs.Parse(args)

	running, message, err := daemon.Status()
	if err != nil {
		log.Fatalf("daemon status failed: %v", err)
	}
	if running {
		log.Printf("daemon running (%s)", message)
	} else {
		log.Printf("daemon not running")
	}

	path := strings.TrimSpace(*configPath)
	if path == "" {
		path = config.DefaultPath()
	}
	loaded, err := config.Load(path)
	if err != nil {
		log.Printf("config load failed: %v", err)
		return
	}
	printTunnelStatus(loaded)
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	configPath := fs.String("config", getenv("PORTOPENER_CONFIG", ""), "config file path")
	url := fs.String("url", getenv("PORTOPENER_RELAY_URL", "ws://localhost/relay"), "relay websocket url")
	fs.Parse(args)

	var token string
	if fs.NArg() > 0 {
		token = fs.Arg(0)
	}
	if strings.TrimSpace(token) == "" {
		log.Fatal("token argument required")
	}

	path := strings.TrimSpace(*configPath)
	if path == "" {
		path = config.DefaultPath()
	}

	current, err := config.Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("config load failed: %v", err)
	}
	if strings.TrimSpace(*url) != "" {
		current.RelayURL = *url
	}
	current.Token = token

	if err := config.Save(path, current); err != nil {
		log.Fatalf("config save failed: %v", err)
	}
	log.Printf("token saved to %s", path)
}

func printUsage() {
	fmt.Println("portopener commands:")
	fmt.Println("  relay --url ws://localhost/relay --token <token>")
	fmt.Println("  http --subdomain <name> --local http://localhost:8081 [--allow <cidr1,cidr2>]")
	fmt.Println("  tcp --external-port <port> --local-host localhost --local-port 8081")
	fmt.Println("  udp --external-port <port> --local-host localhost --local-port 8081")
	fmt.Println("  start --config /path/to/config.json")
	fmt.Println("  daemon start|stop|status [--config /path/to/config.json]")
	fmt.Println("  init <token> [--url ws://localhost/relay] [--config /path/to/config.json]")
}

func runTunnelLoop(ctx context.Context, cfg config.Config, tunnel config.Tunnel) {
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		client := relayclient.New(relayclient.Config{
			URL:          cfg.RelayURL,
			Token:        cfg.Token,
			LocalBaseURL: tunnel.LocalURL,
			LocalHost:    tunnel.LocalHost,
			LocalPort:    tunnel.LocalPort,
		})
		var err error
		switch strings.ToLower(strings.TrimSpace(tunnel.Protocol)) {
		case "http":
			err = client.RegisterHTTP(ctx, tunnel.Subdomain, tunnel.Allowlist)
		case "tcp":
			client.AddStreamHandler("tcp", client.HandleTCPStream)
			err = client.RegisterTCP(ctx, tunnel.ExternalPort)
		case "udp":
			err = client.RegisterUDP(ctx, tunnel.ExternalPort)
		default:
			log.Printf("unknown protocol %q", tunnel.Protocol)
			return
		}
		if err == nil || errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("tunnel %s disconnected: %v", tunnel.Name, err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
		}
	}
}

func resolveToken(token string) string {
	trimmed := strings.TrimSpace(token)
	if trimmed != "" {
		return trimmed
	}
	path := getenv("PORTOPENER_CONFIG", "")
	if strings.TrimSpace(path) == "" {
		path = config.DefaultPath()
	}
	loaded, err := config.Load(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(loaded.Token)
}

func printTunnelStatus(cfg config.Config) {
	if len(cfg.Tunnels) == 0 {
		return
	}
	for _, tunnel := range cfg.Tunnels {
		line := formatTunnelStatus(cfg.PublicBase, tunnel)
		if line != "" {
			log.Printf("tunnel %s => %s", tunnel.Name, line)
		}
	}
}

func formatTunnelStatus(publicBase string, tunnel config.Tunnel) string {
	proto := strings.ToLower(strings.TrimSpace(tunnel.Protocol))
	baseHost, baseScheme := splitPublicBase(publicBase)
	switch proto {
	case "http":
		if tunnel.Subdomain == "" {
			return ""
		}
		if baseHost == "" {
			return tunnel.Subdomain
		}
		urlHost := fmt.Sprintf("%s.%s", tunnel.Subdomain, baseHost)
		if baseScheme != "" {
			return fmt.Sprintf("%s://%s", baseScheme, urlHost)
		}
		return urlHost
	case "tcp", "udp":
		if tunnel.ExternalPort == 0 {
			return ""
		}
		if baseHost == "" {
			return fmt.Sprintf(":%d", tunnel.ExternalPort)
		}
		return fmt.Sprintf("%s:%d", baseHost, tunnel.ExternalPort)
	default:
		return ""
	}
}

func splitPublicBase(publicBase string) (string, string) {
	trimmed := strings.TrimSpace(publicBase)
	if trimmed == "" {
		return "", ""
	}
	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err == nil {
			host := strings.TrimSpace(parsed.Host)
			return host, strings.TrimSpace(parsed.Scheme)
		}
	}
	return strings.TrimRight(trimmed, "/"), ""
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return fallback
}
