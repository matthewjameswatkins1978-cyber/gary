package yard

// ProfileInfo is the non-secret, versioned worker inventory exposed to
// mechanical tooling. Dynamic quota, balance and health belong outside Git.
type ProfileInfo struct {
	ID                 string   `json:"id"`
	Provider           string   `json:"provider"`
	Model              string   `json:"model"`
	Harness            string   `json:"harness"`
	Protocol           string   `json:"protocol"`
	Executable         string   `json:"executable"`
	Capabilities       []string `json:"capabilities"`
	BillingClass       string   `json:"billing_class"`
	Certification      string   `json:"certification"`
	IdentityConfidence string   `json:"identity_confidence"`
	WebSearch          string   `json:"web_search"`
	Limitations        []string `json:"limitations,omitempty"`
}

// Profiles returns the known routes without reading or exposing credentials.
// Candidate routes remain explicit-only until a complete canary is accepted.
func Profiles() []ProfileInfo {
	profiles := []ProfileInfo{
		{ID: "gpt-5.6-luna", Provider: "OpenAI", Model: "gpt-5.6-luna", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "included", Certification: "CERTIFIED", IdentityConfidence: "HIGH", WebSearch: "route-dependent"},
		{ID: "gpt-6-astra", Provider: "OpenAI", Model: "gpt-6-astra", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "included", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "route-dependent"},
		{ID: "gpt-5.6-sol", Provider: "OpenAI", Model: "gpt-5.6-sol", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "included", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "route-dependent"},
		{ID: "gpt-5.6-terra", Provider: "OpenAI", Model: "gpt-5.6-terra", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "included", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "route-dependent"},
		{ID: "gpt-5.5", Provider: "OpenAI", Model: "gpt-5.5", Harness: "Codex CLI", Protocol: "codex-native", Executable: "codex", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "included", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "route-dependent"},
		{ID: "mimo-v2.5", Provider: "Xiaomi MiMo", Model: "mimo-v2.5", Harness: "Codex CLI", Protocol: "responses", Executable: "codex", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "Token Plan", Certification: "CANDIDATE", IdentityConfidence: "LOW", WebSearch: "unavailable", Limitations: []string{"requires process MIMO_API_KEY", "provider metadata warning"}},
		{ID: "opencode-go/muse-spark-1.3-contributor", Provider: "OpenCode Go", Model: "muse-spark-1.3-contributor", Harness: "OpenCode CLI", Protocol: "provider-managed", Executable: "opencode", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "unknown", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "unknown"},
		{ID: "deepseek/deepseek-v4-flash", Provider: "DeepSeek", Model: "deepseek-v4-flash", Harness: "OpenCode CLI", Protocol: "provider-managed", Executable: "opencode", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "unknown", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "unknown"},
		{ID: "cheaper-inference/deepseek-v4-flash", Provider: "cheaper-inference", Model: "deepseek-v4-flash", Harness: "OpenCode CLI", Protocol: "provider-managed", Executable: "opencode", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "unknown", Certification: "CANDIDATE", IdentityConfidence: "UNVERIFIED", WebSearch: "unknown"},
	}
	for _, tier := range []string{"high", "medium", "low"} {
		profiles = append(profiles, ProfileInfo{ID: "antigravity/gemini-3.8-flash-" + tier, Provider: "Google Antigravity", Model: "gemini-3.8-flash-" + tier, Harness: "AGY CLI", Protocol: "local-cli", Executable: "agy", Capabilities: []string{"filesystem", "shell", "git"}, BillingClass: "included quota", Certification: "CANDIDATE", IdentityConfidence: "LOCAL_SESSION", WebSearch: "unknown", Limitations: []string{"local session identity; no route canary yet"}})
	}
	return profiles
}
