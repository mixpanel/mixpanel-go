package mixpanel

import "regexp"

// BotEntry represents a single AI bot pattern in the database.
type BotEntry struct {
	Pattern     *regexp.Regexp
	Name        string
	Provider    string
	Category    string // "indexing", "retrieval", or "agent"
	Description string
}

// BotClassification holds the result of classifying a user-agent string.
type BotClassification struct {
	IsAIBot  bool
	BotName  string
	Provider string
	Category string
}

// aiBotDatabase is the built-in database of known AI bot user-agent patterns.
var aiBotDatabase = []BotEntry{
	{regexp.MustCompile(`(?i)GPTBot/`), "GPTBot", "OpenAI", "indexing", "OpenAI web crawler for model training data"},
	{regexp.MustCompile(`(?i)ChatGPT-User/`), "ChatGPT-User", "OpenAI", "retrieval", "ChatGPT real-time retrieval for user queries (RAG)"},
	{regexp.MustCompile(`(?i)OAI-SearchBot/`), "OAI-SearchBot", "OpenAI", "indexing", "OpenAI search indexing crawler"},
	{regexp.MustCompile(`(?i)ClaudeBot/`), "ClaudeBot", "Anthropic", "indexing", "Anthropic web crawler for model training"},
	{regexp.MustCompile(`(?i)Claude-User/`), "Claude-User", "Anthropic", "retrieval", "Claude real-time retrieval for user queries"},
	{regexp.MustCompile(`(?i)Google-Extended/`), "Google-Extended", "Google", "indexing", "Google AI training data crawler"},
	{regexp.MustCompile(`(?i)PerplexityBot/`), "PerplexityBot", "Perplexity", "retrieval", "Perplexity AI search crawler"},
	{regexp.MustCompile(`(?i)Bytespider/`), "Bytespider", "ByteDance", "indexing", "ByteDance/TikTok AI crawler"},
	{regexp.MustCompile(`(?i)CCBot/`), "CCBot", "Common Crawl", "indexing", "Common Crawl bot (data used by many AI models)"},
	{regexp.MustCompile(`(?i)Applebot-Extended/`), "Applebot-Extended", "Apple", "indexing", "Apple AI/Siri training data crawler"},
	{regexp.MustCompile(`(?i)Meta-ExternalAgent/`), "Meta-ExternalAgent", "Meta", "indexing", "Meta/Facebook AI training data crawler"},
	{regexp.MustCompile(`(?i)cohere-ai/`), "cohere-ai", "Cohere", "indexing", "Cohere AI training data crawler"},
}

// Classifier performs user-agent classification against a bot database.
type Classifier struct {
	bots []BotEntry
}

// NewClassifier creates a Classifier with optional additional bot patterns.
// Additional bots are checked before the built-in database, allowing overrides.
func NewClassifier(additionalBots []BotEntry) *Classifier {
	combined := make([]BotEntry, 0, len(additionalBots)+len(aiBotDatabase))
	combined = append(combined, additionalBots...)
	combined = append(combined, aiBotDatabase...)
	return &Classifier{bots: combined}
}

// Classify checks a user-agent string against this classifier's bot database.
func (c *Classifier) Classify(userAgent string) BotClassification {
	if userAgent == "" {
		return BotClassification{}
	}
	for _, bot := range c.bots {
		if bot.Pattern.MatchString(userAgent) {
			return BotClassification{
				IsAIBot:  true,
				BotName:  bot.Name,
				Provider: bot.Provider,
				Category: bot.Category,
			}
		}
	}
	return BotClassification{}
}

var defaultClassifier = NewClassifier(nil)

// ClassifyUserAgent classifies a user-agent string against the built-in AI bot database.
func ClassifyUserAgent(userAgent string) BotClassification {
	return defaultClassifier.Classify(userAgent)
}

// GetBotDatabase returns a copy of the built-in AI bot database for inspection.
func GetBotDatabase() []BotEntry {
	result := make([]BotEntry, len(aiBotDatabase))
	copy(result, aiBotDatabase)
	return result
}
