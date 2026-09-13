# Python agent example

A runnable template showing how to send OpenTelemetry GenAI traces to Ember
from Python. The OTel wiring is production-shaped; only `call_model()` is a
stub for you to replace with a real provider call.

```bash
pip install -r requirements.txt

export EMBER_ENDPOINT=http://localhost:8080     # default
export EMBER_API_KEY=ember_...                  # from your .env

python agent.py
```

It emits three traces in one session, each with a parent `agent.run` span, a
`execute_tool` span, and a `chat` span carrying model and token attributes.
Open the dashboard and they appear within a couple of seconds.

To use it for real, replace `call_model()` with your provider call and return
the model name and the token counts from the response. Keep the call inside
the `chat` span — that span's duration is what Ember reports as model latency.

See [`docs/INTEGRATIONS.md`](../../docs/INTEGRATIONS.md) for the full
attribute reference, auto-instrumentation options that require no span code
at all, and troubleshooting.
