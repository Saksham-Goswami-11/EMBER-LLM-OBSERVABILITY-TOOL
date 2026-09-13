# Connecting your app to Ember

Ember is a **passive receiver**. It never reaches into your app or calls your
provider. Your app *pushes* OpenTelemetry spans to it over OTLP/HTTP, and
anything that already speaks the OpenTelemetry GenAI conventions works with
no Ember-specific SDK.

Two things are needed, wherever your app runs:

| | |
|---|---|
| **Endpoint** | `http://<ember-host>:8080/v1/traces` |
| **Auth header** | `Authorization: Bearer <your EMBER_API_KEY>` |

That's the whole contract. Pick whichever option below matches how much you
want to change your code.

---

## Option A — environment variables only (no code changes)

If your app already initializes an OpenTelemetry SDK, you usually don't need
to touch it. The standard OTLP variables are enough:

```bash
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:8080/v1/traces
export OTEL_EXPORTER_OTLP_TRACES_HEADERS="Authorization=Bearer%20ember_your_key_here"
export OTEL_SERVICE_NAME=my-agent
```

> **The `%20` matters.** `OTEL_*_HEADERS` is a URL-encoded `key=value` list,
> so the space between `Bearer` and the key must be written `%20`. A literal
> space is the single most common reason traces silently fail to arrive.

Ember's OTLP endpoint accepts both protobuf and JSON over HTTP. If your SDK
defaults to gRPC, set `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=http/protobuf`.

For Python, the zero-code path is:

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install     # adds instrumentation for installed libs
opentelemetry-instrument python my_agent.py
```

## Option B — auto-instrumentation for LLM libraries

If you want spans for your provider calls without writing any yourself, add
a GenAI auto-instrumentation package. Both of these emit the `gen_ai.*`
attributes Ember reads, and both cover OpenAI, Anthropic, LangChain, and
LlamaIndex:

- **OpenLLMetry** (`traceloop-sdk`) — set `TRACELOOP_BASE_URL=http://localhost:8080`
  and `TRACELOOP_HEADERS="Authorization=Bearer%20<key>"`. It appends
  `/v1/traces` itself, so give it the base URL, not the full path.
- **OpenInference** (`openinference-instrumentation-*`) — instrument your
  client, then export with the standard OTLP variables from Option A.

Check the current docs for whichever you choose; their initialization APIs
change more often than the OTLP variables do.

## Option C — write the spans yourself

Full control, no extra dependency beyond the OTel SDK. Two complete, runnable
examples live in this repo:

- [`examples/python-agent`](../examples/python-agent) — Python
- [`examples/demo-agent`](../examples/demo-agent) — Go

The shape that matters:

```python
with tracer.start_as_current_span("agent.run", attributes={"session.id": sid}) as root:
    with tracer.start_as_current_span("chat", attributes={
        "gen_ai.operation.name": "chat",
        "session.id": sid,
    }) as chat:
        response = client.messages.create(...)   # the call goes INSIDE the span
        chat.update_name(f"chat {model}")
        chat.set_attribute("gen_ai.system", "anthropic")
        chat.set_attribute("gen_ai.request.model", model)
        chat.set_attribute("gen_ai.usage.input_tokens", response.usage.input_tokens)
        chat.set_attribute("gen_ai.usage.output_tokens", response.usage.output_tokens)
```

Two things people get wrong here:

1. **Make the provider call inside the span.** The span's duration is what
   Ember charts as model latency. Calling first and opening a span after
   records a 0 ms call.
2. **Flush before a short script exits.** `BatchSpanProcessor` exports
   asynchronously; call `provider.shutdown()` (Go: `provider.Shutdown(ctx)`)
   or the process ends before anything is sent.

---

## What Ember reads off your spans

Everything is optional — a span with none of these still appears in the trace
list — but each one lights up part of the UI.

| Attribute | Effect if present | Effect if missing |
|---|---|---|
| `gen_ai.request.model` / `gen_ai.response.model` | Model column, per-model analytics | Trace shows no model |
| `gen_ai.usage.input_tokens` | Token counts and cost | No usage or spend |
| `gen_ai.usage.output_tokens` | Token counts and cost | No usage or spend |
| `gen_ai.system` | Provider label (`openai`, `anthropic`, …) | Blank provider |
| `gen_ai.operation.name` | Distinguishes `chat` from `execute_tool` | Span typed as generic |
| `session.id` or `gen_ai.conversation.id` | Groups traces into a conversation | Trace stands alone |
| `gen_ai.tool.name` | Names the tool on tool spans | Unnamed tool span |

Legacy names are accepted too: `gen_ai.usage.prompt_tokens` and
`gen_ai.usage.completion_tokens` map to input and output respectively, so
older instrumentation works unchanged.

**Cost** is computed by Ember, not read from your spans — it multiplies your
token counts by the rate table in `internal/pricing/pricing.go`. That table
is hand-maintained and will drift from vendor pricing; a model with no entry
costs `$0.00`. Override it with a JSON file and `EMBER_PRICING_PATH` (see
`examples/demo-agent/pricing.example.json`).

## Reaching Ember from another container

`localhost` inside a container means *that* container. If your app runs in
Docker too:

- **Same compose file** — add your service alongside `ember` and use the
  service name as the host: `http://ember:8080/v1/traces`. Add
  `depends_on: {ember: {condition: service_healthy}}` to wait for it.
- **A different compose project** — put both on a shared external network,
  or publish Ember's port and use `http://host.docker.internal:8080/v1/traces`
  (Docker Desktop on macOS and Windows).
- **A remote host** — use its address, and read the security note below
  first.

## Multiple projects on one Ember

Ember creates a single project, `default`. The ingest endpoint also honors an
`X-Ember-Project` header, but other projects have to exist in the database
first, and v1 ships no UI or command to create them. In practice: run one
Ember per project for now, with a separate `EMBER_DB_PATH` and port.

## Troubleshooting

**Nothing appears in the dashboard.** Check Ember's own log first — it logs
every request:

```bash
docker compose logs -f ember | grep /v1/traces
```

- `POST /v1/traces 401` — the key is wrong. Note that OTel exporters
  **swallow this silently**: your app prints no error and looks like it
  worked. Compare the key against `.env`, and check for a literal space
  instead of `%20` in the headers variable.
- `POST /v1/traces 200` but an empty dashboard — spans arrived but the
  filters may not match. Check `curl http://localhost:8080/api/traces`
  directly.
- No log line at all — the request never arrived. Wrong host or port,
  a container-networking issue (see above), or the app exited before
  flushing.

**Traces appear but tokens and cost are zero.** Your spans lack
`gen_ai.usage.*`, or the model has no entry in the pricing table.

**Spans are not nested in the waterfall.** Child spans must be created inside
the parent's active context. In Python that means nesting
`start_as_current_span` blocks; in Go, passing the context returned by
`tracer.Start`.

**You changed `EMBER_API_KEY` in `.env` and nothing happened.** Compose only
applies it on container recreation:

```bash
docker compose up -d --force-recreate
```

Ember logs `updated the 'default' project API key from EMBER_API_KEY` when it
takes effect.

**You lost a generated key.** It's stored only as a hash, but you no longer
have to delete the database — issue a new one, keeping all your traces:

```bash
docker compose run --rm ember -rotate-key
```

## Security

`/v1/traces` is the only endpoint that checks the API key. The dashboard and
all of `/api/*` are **unauthenticated** — anyone who can reach the port can
read every trace, including prompt metadata. Keep Ember on localhost or a
private network, or put it behind a reverse proxy that enforces your own auth,
before exposing it anywhere shared.
