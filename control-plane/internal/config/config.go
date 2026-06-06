package config

import (
	"encoding/base64"
	"log"
	"os"
	"strconv"
)

// Defaults for Apple Music token lifetime and refresh behaviour.
const (
	defaultAppleTokenTTLSeconds           = 30 * 24 * 60 * 60 // 30 days
	defaultAppleTokenRefreshBufferSeconds = 24 * 60 * 60      // 24 hours

	defaultOpenAITTSModel       = "gpt-4o-mini-tts"
	defaultOpenAITTSVoice       = "nova"
	defaultOpenAITranslateModel = "gpt-4o-mini"
	defaultOpenAITTSFormat      = "mp3" // universal-compat default; flip to "opus" later
	defaultTTSCacheDir          = "/var/cache/whats-tts"
)

type Config struct {
	Port               string
	GatewayInternalURL string

	// Apple Music integration. All three identity fields must be set for the
	// integration to be active; if any is missing, the endpoint serves 503 and
	// the server still boots (WebRTC must remain available).
	AppleTeamID                    string
	AppleKeyID                     string
	ApplePrivateKeyPEM             []byte
	AppleTokenTTLSeconds           int
	AppleTokenRefreshBufferSeconds int

	// OpenAI TTS integration. When OpenAIAPIKey is empty, /v1/tts/enunciate
	// serves 503 and the server still boots.
	OpenAIAPIKey         string
	OpenAITTSModel       string
	OpenAITTSVoice       string
	OpenAITranslateModel string
	OpenAITTSFormat      string // "mp3" or "opus"
	TTSCacheDir          string
}

func Load() *Config {
	cfg := &Config{
		Port:                           getEnv("PORT", "8080"),
		GatewayInternalURL:             getEnv("GATEWAY_INTERNAL_URL", "http://localhost:9091"),
		AppleTeamID:                    os.Getenv("APPLE_TEAM_ID"),
		AppleKeyID:                     os.Getenv("APPLE_KEY_ID"),
		AppleTokenTTLSeconds:           getEnvInt("APPLE_TOKEN_TTL_SECONDS", defaultAppleTokenTTLSeconds),
		AppleTokenRefreshBufferSeconds: getEnvInt("APPLE_TOKEN_REFRESH_BUFFER_SECONDS", defaultAppleTokenRefreshBufferSeconds),

		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		OpenAITTSModel:       getEnv("OPENAI_TTS_MODEL", defaultOpenAITTSModel),
		OpenAITTSVoice:       getEnv("OPENAI_TTS_VOICE", defaultOpenAITTSVoice),
		OpenAITranslateModel: getEnv("OPENAI_TRANSLATE_MODEL", defaultOpenAITranslateModel),
		OpenAITTSFormat:      getEnv("OPENAI_TTS_FORMAT", defaultOpenAITTSFormat),
		TTSCacheDir:          getEnv("TTS_CACHE_DIR", defaultTTSCacheDir),
	}
	cfg.ApplePrivateKeyPEM = loadApplePrivateKey()
	validateAppleConfig(cfg)
	validateOpenAIConfig(cfg)
	return cfg
}

// validateOpenAIConfig warns when the API key is missing. It never fails
// startup — the control-plane must boot so WebRTC and Apple Music keep working
// for the local-file mode.
func validateOpenAIConfig(cfg *Config) {
	if cfg.OpenAIAPIKey == "" {
		log.Printf("config: openai tts disabled — OPENAI_API_KEY not set")
		return
	}
	if cfg.OpenAITTSFormat != "mp3" && cfg.OpenAITTSFormat != "opus" {
		log.Printf("config: OPENAI_TTS_FORMAT=%q unsupported; falling back to %q", cfg.OpenAITTSFormat, defaultOpenAITTSFormat)
		cfg.OpenAITTSFormat = defaultOpenAITTSFormat
	}
}

// loadApplePrivateKey prefers an inline base64 env (prod) over a file path (dev).
// Returns nil when neither is set — the caller treats that as "disabled".
func loadApplePrivateKey() []byte {
	if b64 := os.Getenv("APPLE_MUSIC_PRIVATE_KEY_BASE64"); b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			log.Printf("config: APPLE_MUSIC_PRIVATE_KEY_BASE64 decode failed: %v; apple music disabled", err)
			return nil
		}
		return decoded
	}
	if path := os.Getenv("APPLE_MUSIC_PRIVATE_KEY_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("config: read APPLE_MUSIC_PRIVATE_KEY_FILE (%s) failed: %v; apple music disabled", path, err)
			return nil
		}
		return data
	}
	return nil
}

// validateAppleConfig warns and disables Apple Music when the three required
// fields are partially populated. It never fails startup — the control-plane
// must still run so WebRTC is available.
func validateAppleConfig(cfg *Config) {
	hasTeam := cfg.AppleTeamID != ""
	hasKey := cfg.AppleKeyID != ""
	hasPEM := len(cfg.ApplePrivateKeyPEM) > 0

	switch {
	case !hasTeam && !hasKey && !hasPEM:
		log.Printf("config: apple music disabled — no key configured")
	case hasTeam && hasKey && hasPEM:
		// Fully configured — nothing to warn about.
	default:
		log.Printf("config: apple music disabled — partial configuration (team=%t key=%t pem=%t)", hasTeam, hasKey, hasPEM)
		// Zero everything out so downstream sees a clean "disabled" state.
		cfg.AppleTeamID = ""
		cfg.AppleKeyID = ""
		cfg.ApplePrivateKeyPEM = nil
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("config: %s=%q is not an integer; using default %d", key, v, fallback)
		return fallback
	}
	return n
}
