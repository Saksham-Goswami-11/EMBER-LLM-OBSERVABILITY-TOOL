// Package ingest implements the OTLP/HTTP traces receiver: the only way
// data gets into Ember. Any OpenTelemetry SDK or Collector exporter that
// can send OTLP/HTTP traces (protobuf or JSON) can point at this endpoint
// with zero Ember-specific code.
package ingest

import (
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"github.com/sakshamgoswami/ember/internal/model"
	"github.com/sakshamgoswami/ember/internal/pricing"
	"github.com/sakshamgoswami/ember/internal/storage"
)

const maxBodyBytes = 32 << 20 // 32MB

type Handler struct {
	Store   *storage.Store
	Pricing *pricing.Table
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectID := r.Header.Get("X-Ember-Project")
	if projectID == "" {
		projectID = "default"
	}
	key := extractAPIKey(r)
	ok, err := h.Store.ValidateAPIKey(r.Context(), projectID, key)
	if err != nil {
		http.Error(w, "internal error validating project", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "unknown project or invalid API key", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	var req coltracepb.ExportTraceServiceRequest
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		err = protojson.Unmarshal(body, &req)
	} else {
		err = proto.Unmarshal(body, &req)
	}
	if err != nil {
		http.Error(w, "malformed OTLP export request: "+err.Error(), http.StatusBadRequest)
		return
	}

	batches := convertToBatches(&req, h.Pricing)
	if len(batches) > 0 {
		if err := h.Store.WriteTraces(r.Context(), projectID, batches); err != nil {
			http.Error(w, "failed to persist spans", http.StatusInternalServerError)
			return
		}
	}

	resp, _ := proto.Marshal(&coltracepb.ExportTraceServiceResponse{})
	w.Header().Set("Content-Type", "application/x-protobuf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}

func extractAPIKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return r.Header.Get("X-Ember-Api-Key")
}

// convertToBatches maps OTLP resource/scope/span structures onto Ember's
// span model and groups the result by trace ID, ready for storage.
func convertToBatches(req *coltracepb.ExportTraceServiceRequest, prices *pricing.Table) []storage.TraceBatch {
	type acc struct {
		sessionID string
		spans     []model.Span
	}
	groups := make(map[string]*acc)

	for _, rs := range req.ResourceSpans {
		resourceAttrs := attrsToMap(nil, rs.GetResource())
		for _, ss := range rs.ScopeSpans {
			for _, pbSpan := range ss.Spans {
				attrs := attrsToMap(resourceAttrs, nil)
				for _, kv := range pbSpan.Attributes {
					attrs[kv.Key] = attrValueToString(kv.Value)
				}

				traceID := hex.EncodeToString(pbSpan.TraceId)
				spanID := hex.EncodeToString(pbSpan.SpanId)
				parentID := ""
				if len(pbSpan.ParentSpanId) > 0 {
					parentID = hex.EncodeToString(pbSpan.ParentSpanId)
				}

				model_ := firstNonEmpty(attrs["gen_ai.response.model"], attrs["gen_ai.request.model"])
				inTok := firstIntAttr(attrs, "gen_ai.usage.input_tokens", "gen_ai.usage.prompt_tokens")
				outTok := firstIntAttr(attrs, "gen_ai.usage.output_tokens", "gen_ai.usage.completion_tokens")
				sessionID := firstNonEmpty(attrs["session.id"], attrs["gen_ai.conversation.id"])

				sp := model.Span{
					ID:            spanID,
					TraceID:       traceID,
					ParentSpanID:  parentID,
					Name:          pbSpan.Name,
					OperationName: attrs["gen_ai.operation.name"],
					System:        attrs["gen_ai.system"],
					Model:         model_,
					InputTokens:   inTok,
					OutputTokens:  outTok,
					CostUSD:       prices.Calculate(model_, inTok, outTok),
					StartedAtMS:   int64(pbSpan.StartTimeUnixNano / 1e6),
					EndedAtMS:     int64(pbSpan.EndTimeUnixNano / 1e6),
					StatusCode:    statusCodeString(pbSpan.GetStatus().GetCode()),
					StatusMessage: pbSpan.GetStatus().GetMessage(),
					Attributes:    attrs,
				}
				sp.DurationMS = sp.EndedAtMS - sp.StartedAtMS
				if sp.DurationMS < 0 {
					sp.DurationMS = 0
				}

				g, ok := groups[traceID]
				if !ok {
					g = &acc{}
					groups[traceID] = g
				}
				if g.sessionID == "" {
					g.sessionID = sessionID
				}
				g.spans = append(g.spans, sp)
			}
		}
	}

	batches := make([]storage.TraceBatch, 0, len(groups))
	for traceID, g := range groups {
		batches = append(batches, storage.TraceBatch{TraceID: traceID, SessionID: g.sessionID, Spans: g.spans})
	}
	return batches
}

func attrsToMap(base map[string]string, resource *resourcepb.Resource) map[string]string {
	out := make(map[string]string, len(base)+8)
	for k, v := range base {
		out[k] = v
	}
	if resource != nil {
		for _, kv := range resource.Attributes {
			out[kv.Key] = attrValueToString(kv.Value)
		}
	}
	return out
}

func attrValueToString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(val.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'f', -1, 64)
	case *commonpb.AnyValue_BytesValue:
		return hex.EncodeToString(val.BytesValue)
	default:
		return ""
	}
}

func statusCodeString(code tracepb.Status_StatusCode) string {
	switch code {
	case tracepb.Status_STATUS_CODE_OK:
		return "ok"
	case tracepb.Status_STATUS_CODE_ERROR:
		return "error"
	default:
		return "unset"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstIntAttr(attrs map[string]string, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := attrs[k]; ok {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}
