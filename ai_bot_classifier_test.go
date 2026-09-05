package mixpanel

import (
	"regexp"
	"testing"
	"github.com/stretchr/testify/require"
)

func TestClassifyUserAgent(t *testing.T) {
	// === OpenAI Bots ===
	t.Run("classifies GPTBot user agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; GPTBot/1.2; +https://openai.com/gptbot)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "GPTBot", result.BotName)
		require.Equal(t, "OpenAI", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})
	t.Run("classifies ChatGPT-User agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ChatGPT-User/1.0; +https://openai.com/bot)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "ChatGPT-User", result.BotName)
		require.Equal(t, "OpenAI", result.Provider)
		require.Equal(t, "retrieval", result.Category)
	})
	t.Run("classifies OAI-SearchBot agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; OAI-SearchBot/1.0; +https://openai.com/searchbot)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "OAI-SearchBot", result.BotName)
		require.Equal(t, "OpenAI", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === Anthropic Bots ===
	t.Run("classifies ClaudeBot agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "ClaudeBot", result.BotName)
		require.Equal(t, "Anthropic", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})
	t.Run("classifies Claude-User agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; Claude-User/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "Claude-User", result.BotName)
		require.Equal(t, "Anthropic", result.Provider)
		require.Equal(t, "retrieval", result.Category)
	})

	// === Google ===
	t.Run("classifies Google-Extended agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; Google-Extended/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "Google-Extended", result.BotName)
		require.Equal(t, "Google", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === Perplexity ===
	t.Run("classifies PerplexityBot agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; PerplexityBot/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "PerplexityBot", result.BotName)
		require.Equal(t, "Perplexity", result.Provider)
		require.Equal(t, "retrieval", result.Category)
	})

	// === ByteDance ===
	t.Run("classifies Bytespider agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; Bytespider/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "Bytespider", result.BotName)
		require.Equal(t, "ByteDance", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === Common Crawl ===
	t.Run("classifies CCBot agent", func(t *testing.T) {
		result := ClassifyUserAgent("CCBot/2.0 (https://commoncrawl.org/faq/)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "CCBot", result.BotName)
		require.Equal(t, "Common Crawl", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === Apple ===
	t.Run("classifies Applebot-Extended agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Applebot-Extended/0.1")
		require.True(t, result.IsAIBot)
		require.Equal(t, "Applebot-Extended", result.BotName)
		require.Equal(t, "Apple", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === Meta ===
	t.Run("classifies Meta-ExternalAgent agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; Meta-ExternalAgent/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "Meta-ExternalAgent", result.BotName)
		require.Equal(t, "Meta", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === Cohere ===
	t.Run("classifies cohere-ai agent", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; cohere-ai/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "cohere-ai", result.BotName)
		require.Equal(t, "Cohere", result.Provider)
		require.Equal(t, "indexing", result.Category)
	})

	// === NEGATIVE CASES ===
	t.Run("does not classify regular Chrome as AI bot", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		require.False(t, result.IsAIBot)
		require.Empty(t, result.BotName)
		require.Empty(t, result.Provider)
		require.Empty(t, result.Category)
	})
	t.Run("does not classify regular Googlebot as AI bot", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
		require.False(t, result.IsAIBot)
	})
	t.Run("does not classify regular Bingbot as AI bot", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)")
		require.False(t, result.IsAIBot)
	})
	t.Run("does not classify curl as AI bot", func(t *testing.T) {
		result := ClassifyUserAgent("curl/7.64.1")
		require.False(t, result.IsAIBot)
	})
	t.Run("handles empty user agent", func(t *testing.T) {
		result := ClassifyUserAgent("")
		require.False(t, result.IsAIBot)
	})

	// === CASE SENSITIVITY ===
	t.Run("matches case-insensitively", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 (compatible; gptbot/1.2)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "GPTBot", result.BotName)
	})

	// === RETURN SHAPE ===
	t.Run("returns all expected fields for a match", func(t *testing.T) {
		result := ClassifyUserAgent("GPTBot/1.2")
		require.True(t, result.IsAIBot)
		require.NotEmpty(t, result.BotName)
		require.NotEmpty(t, result.Provider)
		require.Contains(t, []string{"indexing", "retrieval", "agent"}, result.Category)
	})
	t.Run("returns zero-value fields for non-matches", func(t *testing.T) {
		result := ClassifyUserAgent("Mozilla/5.0 Chrome/120")
		require.False(t, result.IsAIBot)
		require.Empty(t, result.BotName)
		require.Empty(t, result.Provider)
		require.Empty(t, result.Category)
	})
}

func TestGetBotDatabase(t *testing.T) {
	t.Run("returns a non-empty slice", func(t *testing.T) {
		db := GetBotDatabase()
		require.NotEmpty(t, db)
		require.GreaterOrEqual(t, len(db), 12)
	})
	t.Run("each entry has required fields", func(t *testing.T) {
		db := GetBotDatabase()
		for _, entry := range db {
			require.NotNil(t, entry.Pattern, "Pattern must not be nil for %s", entry.Name)
			require.NotEmpty(t, entry.Name)
			require.NotEmpty(t, entry.Provider)
			require.Contains(t, []string{"indexing", "retrieval", "agent"}, entry.Category)
		}
	})
}

func TestNewClassifier(t *testing.T) {
	t.Run("custom bot patterns are checked", func(t *testing.T) {
		classifier := NewClassifier([]BotEntry{{
			Pattern:  regexp.MustCompile(`(?i)MyCustomBot/`),
			Name:     "MyCustomBot",
			Provider: "CustomCorp",
			Category: "indexing",
		}})
		result := classifier.Classify("Mozilla/5.0 (compatible; MyCustomBot/1.0)")
		require.True(t, result.IsAIBot)
		require.Equal(t, "MyCustomBot", result.BotName)
		require.Equal(t, "CustomCorp", result.Provider)
	})
	t.Run("custom bots are checked before built-in bots", func(t *testing.T) {
		classifier := NewClassifier([]BotEntry{{
			Pattern:  regexp.MustCompile(`(?i)GPTBot/`),
			Name:     "GPTBot-Custom",
			Provider: "CustomProvider",
			Category: "retrieval",
		}})
		result := classifier.Classify("GPTBot/1.2")
		require.Equal(t, "GPTBot-Custom", result.BotName)
		require.Equal(t, "CustomProvider", result.Provider)
		require.Equal(t, "retrieval", result.Category)
	})
	t.Run("built-in bots still work with additional bots", func(t *testing.T) {
		classifier := NewClassifier([]BotEntry{{
			Pattern:  regexp.MustCompile(`(?i)MyBot/`),
			Name:     "MyBot",
			Provider: "MyCorp",
			Category: "indexing",
		}})
		result := classifier.Classify("ClaudeBot/1.0")
		require.True(t, result.IsAIBot)
		require.Equal(t, "ClaudeBot", result.BotName)
	})
	t.Run("nil additional bots uses defaults only", func(t *testing.T) {
		classifier := NewClassifier(nil)
		result := classifier.Classify("GPTBot/1.2")
		require.True(t, result.IsAIBot)
		require.Equal(t, "GPTBot", result.BotName)
	})
}
