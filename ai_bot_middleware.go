package mixpanel

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type contextKey string

const botClassificationKey contextKey = "mixpanel_bot_classification"

// BotClassifyingIngestion wraps an Ingestion implementation and enriches
// Track() calls with AI bot classification properties when $user_agent
// is present in event properties. All other methods delegate unchanged.
type BotClassifyingIngestion struct {
	inner      Ingestion
	classifier *Classifier
}

var _ Ingestion = (*BotClassifyingIngestion)(nil)

// NewBotClassifyingIngestion creates a wrapper using the built-in bot database.
func NewBotClassifyingIngestion(inner Ingestion) *BotClassifyingIngestion {
	return &BotClassifyingIngestion{inner: inner, classifier: defaultClassifier}
}

// NewBotClassifyingIngestionWithClassifier creates a wrapper with a custom Classifier.
func NewBotClassifyingIngestionWithClassifier(inner Ingestion, classifier *Classifier) *BotClassifyingIngestion {
	return &BotClassifyingIngestion{inner: inner, classifier: classifier}
}

// Track classifies events containing $user_agent and injects bot properties
// before forwarding to the inner Ingestion. If $user_agent matches an AI bot:
// sets $is_ai_bot=true, $ai_bot_name, $ai_bot_provider, $ai_bot_category.
// If present but no match: $is_ai_bot=false. If absent: no properties added.
func (b *BotClassifyingIngestion) Track(ctx context.Context, events []*Event) error {
	for _, event := range events {
		if event.Properties == nil {
			continue
		}
		ua, ok := event.Properties["$user_agent"]
		if !ok {
			continue
		}
		uaStr, ok := ua.(string)
		if !ok {
			continue
		}
		classification := b.classifier.Classify(uaStr)
		event.Properties["$is_ai_bot"] = classification.IsAIBot
		if classification.IsAIBot {
			event.Properties["$ai_bot_name"] = classification.BotName
			event.Properties["$ai_bot_provider"] = classification.Provider
			event.Properties["$ai_bot_category"] = classification.Category
		}
	}
	return b.inner.Track(ctx, events)
}

// --- Delegated Ingestion methods (all pass through unchanged) ---

func (b *BotClassifyingIngestion) Import(ctx context.Context, events []*Event, options ImportOptions) (*ImportSuccess, error) {
	return b.inner.Import(ctx, events, options)
}
func (b *BotClassifyingIngestion) PeopleSet(ctx context.Context, people []*PeopleProperties) error {
	return b.inner.PeopleSet(ctx, people)
}
func (b *BotClassifyingIngestion) PeopleSetOnce(ctx context.Context, people []*PeopleProperties) error {
	return b.inner.PeopleSetOnce(ctx, people)
}
func (b *BotClassifyingIngestion) PeopleIncrement(ctx context.Context, distinctID string, add map[string]int) error {
	return b.inner.PeopleIncrement(ctx, distinctID, add)
}
func (b *BotClassifyingIngestion) PeopleUnionProperty(ctx context.Context, distinctID string, union map[string]any) error {
	return b.inner.PeopleUnionProperty(ctx, distinctID, union)
}
func (b *BotClassifyingIngestion) PeopleAppendListProperty(ctx context.Context, distinctID string, appendMap map[string]any) error {
	return b.inner.PeopleAppendListProperty(ctx, distinctID, appendMap)
}
func (b *BotClassifyingIngestion) PeopleRemoveListProperty(ctx context.Context, distinctID string, remove map[string]any) error {
	return b.inner.PeopleRemoveListProperty(ctx, distinctID, remove)
}
func (b *BotClassifyingIngestion) PeopleDeleteProperty(ctx context.Context, distinctID string, unset []string) error {
	return b.inner.PeopleDeleteProperty(ctx, distinctID, unset)
}
func (b *BotClassifyingIngestion) PeopleDeleteProfile(ctx context.Context, distinctID string, ignoreAlias bool) error {
	return b.inner.PeopleDeleteProfile(ctx, distinctID, ignoreAlias)
}
func (b *BotClassifyingIngestion) GroupSet(ctx context.Context, groupKey, groupID string, set map[string]any) error {
	return b.inner.GroupSet(ctx, groupKey, groupID, set)
}
func (b *BotClassifyingIngestion) GroupSetOnce(ctx context.Context, groupKey, groupID string, set map[string]any) error {
	return b.inner.GroupSetOnce(ctx, groupKey, groupID, set)
}
func (b *BotClassifyingIngestion) GroupDeleteProperty(ctx context.Context, groupKey, groupID string, unset []string) error {
	return b.inner.GroupDeleteProperty(ctx, groupKey, groupID, unset)
}
func (b *BotClassifyingIngestion) GroupRemoveListProperty(ctx context.Context, groupKey, groupID string, remove map[string]any) error {
	return b.inner.GroupRemoveListProperty(ctx, groupKey, groupID, remove)
}
func (b *BotClassifyingIngestion) GroupUnionListProperty(ctx context.Context, groupKey, groupID string, union map[string]any) error {
	return b.inner.GroupUnionListProperty(ctx, groupKey, groupID, union)
}
func (b *BotClassifyingIngestion) GroupDelete(ctx context.Context, groupKey, groupID string) error {
	return b.inner.GroupDelete(ctx, groupKey, groupID)
}

// --- HTTP Middleware ---

// BotClassificationMiddleware returns an http.Handler middleware that classifies
// the request's User-Agent and stores the result in context.
func BotClassificationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		classification := ClassifyUserAgent(r.UserAgent())
		ctx := context.WithValue(r.Context(), botClassificationKey, &classification)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BotClassificationFromContext retrieves the BotClassification stored by
// BotClassificationMiddleware. Returns nil if the middleware was not applied.
func BotClassificationFromContext(ctx context.Context) *BotClassification {
	val := ctx.Value(botClassificationKey)
	if val == nil {
		return nil
	}
	bc, ok := val.(*BotClassification)
	if !ok {
		return nil
	}
	return bc
}

// --- TrackRequest Helper ---

// TrackRequest extracts User-Agent and client IP from an *http.Request and
// sets them on the event's Properties map ($user_agent, ip).
func TrackRequest(r *http.Request, event *Event) {
	if event.Properties == nil {
		event.Properties = make(map[string]any)
	}
	if ua := r.UserAgent(); ua != "" {
		event.Properties["$user_agent"] = ua
	}
	if ip := extractClientIP(r); ip != "" {
		event.AddIP(net.ParseIP(ip))
	}
}

// extractClientIP gets client IP from X-Forwarded-For (first entry) or RemoteAddr.
func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
