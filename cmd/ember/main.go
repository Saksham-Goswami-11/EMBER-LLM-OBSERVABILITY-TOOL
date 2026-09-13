// Command ember runs the whole product: OTLP ingestion, the query API,
// and the dashboard, as one process with one embedded SQLite database.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sakshamgoswami/ember/internal/pricing"
	"github.com/sakshamgoswami/ember/internal/server"
	"github.com/sakshamgoswami/ember/internal/storage"
	"github.com/sakshamgoswami/ember/web"
)

func main() {
	addr := flag.String("addr", envOr("EMBER_ADDR", ":8080"), "address to listen on")
	dbPath := flag.String("db", envOr("EMBER_DB_PATH", "ember.db"), "path to the SQLite database file")
	pricingPath := flag.String("pricing", os.Getenv("EMBER_PRICING_PATH"), "optional JSON file of model pricing overrides")
	rotateKey := flag.Bool("rotate-key", false, "generate a new API key for the default project, print it, and exit")
	healthcheck := flag.Bool("healthcheck", false, "probe a running ember at -addr and exit 0 if healthy (used by the container healthcheck)")
	flag.Parse()

	// Must run before the database is opened: the runtime image is distroless
	// and has no shell or curl, so the binary is its own health probe.
	if *healthcheck {
		if err := probeHealth(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "ember: healthcheck failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	store, err := storage.Open(*dbPath)
	if err != nil {
		log.Fatalf("ember: failed to open database %q: %v", *dbPath, err)
	}
	defer store.Close()

	prices, err := pricing.Load(*pricingPath)
	if err != nil {
		log.Fatalf("ember: failed to load pricing table: %v", err)
	}

	ctx := context.Background()

	if *rotateKey {
		if _, _, err := store.EnsureProject(ctx, "default", "Default Project", ""); err != nil {
			log.Fatalf("ember: failed to set up default project: %v", err)
		}
		newKey, err := store.SetAPIKey(ctx, "default", "")
		if err != nil {
			log.Fatalf("ember: failed to rotate API key: %v", err)
		}
		printKeyBanner("Ember rotated the API key for the 'default' project.", newKey, *addr)
		return
	}

	// EMBER_API_KEY lets an operator pin the key up front (this is what the
	// compose setup does) instead of scraping it out of the first-run logs.
	envKey := os.Getenv("EMBER_API_KEY")
	if envKey != "" && !storage.ValidAPIKeyFormat(envKey) {
		log.Fatalf("ember: EMBER_API_KEY is set but not usable (need 16-200 chars, no surrounding whitespace)")
	}

	key, created, err := store.EnsureProject(ctx, "default", "Default Project", envKey)
	if err != nil {
		log.Fatalf("ember: failed to set up default project: %v", err)
	}
	switch {
	case created && envKey != "":
		log.Println("ember: created the 'default' project using the key from EMBER_API_KEY")
	case created:
		printKeyBanner("Ember generated an API key for the 'default' project.", key, *addr)
	case envKey != "":
		// The project predates this key. Adopt it so editing .env actually
		// takes effect, but stay quiet when it already matches.
		matches, err := store.ValidateAPIKey(ctx, "default", envKey)
		if err != nil {
			log.Fatalf("ember: failed to check the existing API key: %v", err)
		}
		if !matches {
			if _, err := store.SetAPIKey(ctx, "default", envKey); err != nil {
				log.Fatalf("ember: failed to apply EMBER_API_KEY: %v", err)
			}
			log.Println("ember: updated the 'default' project API key from EMBER_API_KEY")
		}
	}

	handler := server.New(store, prices, web.FS())
	httpServer := &http.Server{
		Addr:         *addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("ember: listening on %s — dashboard at http://localhost%s", *addr, displayAddr(*addr))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ember: server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("ember: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

// probeHealth GETs /healthz on a running instance. addr is a listen address
// (":8080"), so an empty or wildcard host becomes localhost for the request.
func probeHealth(addr string) error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://localhost" + displayAddr(addr) + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}

func printKeyBanner(headline, key, addr string) {
	fmt.Println("=========================================================")
	fmt.Printf(" %s\n", headline)
	fmt.Println(" It's stored as a hash — this is the only time it's shown.")
	fmt.Println(" Lost it? Run 'ember -rotate-key' to issue a new one.")
	fmt.Println()
	fmt.Printf("   %s\n", key)
	fmt.Println()
	fmt.Println(" Point an OTel exporter at this instance:")
	fmt.Printf("   OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost%s/v1/traces\n", displayAddr(addr))
	fmt.Printf("   OTEL_EXPORTER_OTLP_TRACES_HEADERS=Authorization=Bearer%%20%s\n", key)
	fmt.Println("=========================================================")
}

// displayAddr reduces a listen address to the ":port" suffix a browser or a
// local probe can be pointed at, so "0.0.0.0:8080" and ":8080" both render
// as ":8080" rather than something unusable.
func displayAddr(addr string) string {
	if _, port, err := net.SplitHostPort(addr); err == nil && port != "" {
		return ":" + port
	}
	if strings.HasPrefix(addr, ":") {
		return addr
	}
	return ":" + addr
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
