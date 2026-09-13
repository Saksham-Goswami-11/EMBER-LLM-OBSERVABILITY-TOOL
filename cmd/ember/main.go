// Command ember runs the whole product: OTLP ingestion, the query API,
// and the dashboard, as one process with one embedded SQLite database.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	flag.Parse()

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
	key, created, err := store.EnsureProject(ctx, "default", "Default Project")
	if err != nil {
		log.Fatalf("ember: failed to set up default project: %v", err)
	}
	if created {
		printFirstRunBanner(key, *addr)
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

func printFirstRunBanner(key, addr string) {
	fmt.Println("=========================================================")
	fmt.Println(" Ember generated an API key for the 'default' project.")
	fmt.Println(" It's stored as a hash — this is the only time it's shown.")
	fmt.Println()
	fmt.Printf("   %s\n", key)
	fmt.Println()
	fmt.Println(" Point an OTel exporter at this instance:")
	fmt.Printf("   OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost%s/v1/traces\n", displayAddr(addr))
	fmt.Printf("   OTEL_EXPORTER_OTLP_TRACES_HEADERS=Authorization=Bearer%%20%s\n", key)
	fmt.Println("=========================================================")
}

func displayAddr(addr string) string {
	if len(addr) > 0 && addr[0] == ':' {
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
