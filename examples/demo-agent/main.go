// Command demo-agent sends realistic-looking OTel GenAI traces to a
// running Ember instance, so a fresh install has something to look at
// immediately. It's plain OpenTelemetry — nothing Ember-specific — which
// is the point: this is what any real agent's instrumentation looks like.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var models = []string{"gpt-4o-mini", "gpt-4o", "claude-3-5-sonnet"}

func main() {
	endpoint := getenv("EMBER_ENDPOINT", "localhost:8080")
	apiKey := os.Getenv("EMBER_API_KEY")
	if apiKey == "" {
		log.Fatal("demo-agent: set EMBER_API_KEY to the key ember printed on first run")
	}
	turns := 16
	if v := os.Getenv("EMBER_DEMO_TURNS"); v != "" {
		fmt.Sscanf(v, "%d", &turns)
	}

	ctx := context.Background()
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithURLPath("/v1/traces"),
		otlptracehttp.WithInsecure(),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": "Bearer " + apiKey}),
	)
	if err != nil {
		log.Fatalf("demo-agent: failed to create exporter: %v", err)
	}

	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(ctx); err != nil {
			log.Printf("demo-agent: shutdown error: %v", err)
		}
	}()

	tracer := otel.Tracer("demo-agent")

	// Three overlapping conversations, several turns each.
	sessionIDs := []string{randID("sess"), randID("sess"), randID("sess")}

	for i := 0; i < turns; i++ {
		session := sessionIDs[i%len(sessionIDs)]
		runTurn(ctx, tracer, session, i)
		time.Sleep(250 * time.Millisecond)
	}

	provider.ForceFlush(ctx)
	fmt.Printf("demo-agent: sent %d traces across %d sessions to %s\n", turns, len(sessionIDs), endpoint)
}

func runTurn(ctx context.Context, tracer trace.Tracer, sessionID string, i int) {
	model := models[i%len(models)]
	system := "openai"
	if model == "claude-3-5-sonnet" {
		system = "anthropic"
	}
	failing := rand.Intn(10) == 0

	ctx, root := tracer.Start(ctx, "agent.run", trace.WithAttributes(
		attribute.String("session.id", sessionID),
	))

	// A little over half the turns need a tool call before the final answer.
	if rand.Intn(2) == 0 {
		toolCtx, toolSpan := tracer.Start(ctx, "execute_tool", trace.WithAttributes(
			attribute.String("gen_ai.operation.name", "execute_tool"),
			attribute.String("gen_ai.tool.name", "web_search"),
			attribute.String("session.id", sessionID),
		))
		sleepJitter(80, 220)
		toolSpan.End()
		ctx = toolCtx
	}

	inTok := int64(120 + rand.Intn(600))
	outTok := int64(40 + rand.Intn(300))
	_, chat := tracer.Start(ctx, fmt.Sprintf("chat %s", model), trace.WithAttributes(
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("gen_ai.system", system),
		attribute.String("gen_ai.request.model", model),
		attribute.String("gen_ai.response.model", model),
		attribute.Int64("gen_ai.usage.input_tokens", inTok),
		attribute.Int64("gen_ai.usage.output_tokens", outTok),
		attribute.String("session.id", sessionID),
	))
	sleepJitter(300, 1400)
	if failing {
		chat.SetStatus(codes.Error, "upstream rate limited")
		root.SetStatus(codes.Error, "upstream rate limited")
	}
	chat.End()
	root.End()
}

func sleepJitter(minMS, maxMS int) {
	time.Sleep(time.Duration(minMS+rand.Intn(maxMS-minMS)) * time.Millisecond)
}

func randID(prefix string) string {
	return fmt.Sprintf("%s-%06d", prefix, rand.Intn(1_000_000))
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
