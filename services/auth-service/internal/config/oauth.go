package config

type OAuthConfig struct {
	Google OAuthProviderConfig `yaml:"google" env-prefix:"GOOGLE_"`
	GitHub OAuthProviderConfig `yaml:"github" env-prefix:"GITHUB_"`
}

type OAuthProviderConfig struct {
	Enabled      bool   `yaml:"enabled"      env-default:"false"`
	ClientID     string `yaml:"clientID"     env:"CLIENT_ID" env-required:"true"`
	ClientSecret string `yaml:"clientSecret" env:"CLIENT_SECRET" env-required:"true"`
	RedirectURL  string `yaml:"redirectURL"  env:"REDIRECT_URL" env-required:"true"`
}
