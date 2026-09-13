"""Send OpenTelemetry GenAI traces to Ember from Python.

This is a template, not a toy: the OpenTelemetry wiring below is exactly what
you would keep in a real app. The only part you replace is `call_model()` —
swap the stub for your actual provider call and the spans stay the same.

    pip install -r requirements.txt
    export EMBER_ENDPOINT=http://localhost:8080
    export EMBER_API_KEY=ember_...
    python agent.py

Prefer not to write span code at all? See docs/INTEGRATIONS.md — auto
instrumentation emits these same attributes with no code changes.
"""

import os
import sys
import time
import uuid

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.trace import Status, StatusCode

ENDPOINT = os.getenv("EMBER_ENDPOINT", "http://localhost:8080").rstrip("/")
API_KEY = os.getenv("EMBER_API_KEY")

if not API_KEY:
    sys.exit("Set EMBER_API_KEY to the key from your .env or Ember's first-run log.")


def configure_tracing(service_name: str = "python-agent") -> trace.Tracer:
    """Point the OTel SDK at Ember. This is the whole integration."""
    provider = TracerProvider(resource=Resource.create({"service.name": service_name}))
    provider.add_span_processor(
        BatchSpanProcessor(
            OTLPSpanExporter(
                endpoint=f"{ENDPOINT}/v1/traces",
                headers={"Authorization": f"Bearer {API_KEY}"},
            )
        )
    )
    trace.set_tracer_provider(provider)
    return trace.get_tracer(__name__)


def call_model(prompt: str):
    """Replace this with a real provider call.

    Return (text, model, system, input_tokens, output_tokens). Every major
    SDK exposes those token counts on the response — for example
    `response.usage.input_tokens` / `response.usage.output_tokens` on
    Anthropic, or `response.usage.prompt_tokens` /
    `response.usage.completion_tokens` on OpenAI.
    """
    time.sleep(0.3)
    return (
        f"(stub answer to {prompt!r} — swap call_model() for your real client)",
        "gpt-4o-mini",
        "openai",
        len(prompt.split()) + 20,
        32,
    )


def run_turn(tracer: trace.Tracer, session_id: str, prompt: str) -> str:
    """One agent turn: a parent span, an optional tool span, and the LLM span.

    The nesting is what produces Ember's waterfall — child spans are created
    inside the parent's context.
    """
    with tracer.start_as_current_span(
        "agent.run",
        # session.id groups separate traces into one conversation in Ember.
        # gen_ai.conversation.id works too, if your stack already sets it.
        attributes={"session.id": session_id},
    ) as root:
        with tracer.start_as_current_span(
            "execute_tool",
            attributes={
                "gen_ai.operation.name": "execute_tool",
                "gen_ai.tool.name": "web_search",
                "session.id": session_id,
            },
        ):
            time.sleep(0.1)

        # The provider call must happen *inside* this span — the span's
        # duration is what Ember charts as model latency. Attributes that
        # are only known from the response are set afterwards.
        with tracer.start_as_current_span(
            "chat",
            attributes={
                "gen_ai.operation.name": "chat",
                "session.id": session_id,
            },
        ) as chat:
            try:
                text, model, system, in_tok, out_tok = call_model(prompt)
            except Exception as exc:
                # Mark both spans so the trace list shows the failure.
                chat.set_status(Status(StatusCode.ERROR, str(exc)))
                root.set_status(Status(StatusCode.ERROR, str(exc)))
                raise

            chat.update_name(f"chat {model}")
            chat.set_attribute("gen_ai.system", system)
            chat.set_attribute("gen_ai.request.model", model)
            chat.set_attribute("gen_ai.response.model", model)
            # These two drive Ember's token and cost columns. Without them a
            # trace still appears, but with no usage or spend attached.
            chat.set_attribute("gen_ai.usage.input_tokens", in_tok)
            chat.set_attribute("gen_ai.usage.output_tokens", out_tok)

        return text


def main() -> None:
    tracer = configure_tracing()
    session_id = f"py-{uuid.uuid4().hex[:8]}"

    for prompt in (
        "What changed in the deploy last night?",
        "Summarize the error budget burn.",
        "Draft a status update for the team.",
    ):
        answer = run_turn(tracer, session_id, prompt)
        print(f"> {prompt}\n  {answer}\n")

    # Flush before exit — BatchSpanProcessor is asynchronous, and a short
    # script will otherwise terminate before anything is exported.
    trace.get_tracer_provider().shutdown()
    print(f"Sent 3 traces (session {session_id}) to {ENDPOINT}")


if __name__ == "__main__":
    main()
