package identity

// The factories below are registration holders. Identity providers are
// provisioned once at boot with platform config (RPName, repos, store). The
// WebAuthn provider no longer holds a fixed RPID — it derives the
// relying-party ID per request from the Host header — so the factory returns
// the prebuilt instance from New(); no per-request work and nothing cached.
// The three capability methods collapse the old authenticator / challenger /
// delegator maps into one name-keyed registry with a capability discriminator.

type passwordFactory struct{ p *PasswordProvider }

// NewPasswordFactory wraps a provisioned password provider as a Factory.
func NewPasswordFactory(p *PasswordProvider) Factory { return passwordFactory{p} }

func (passwordFactory) Name() string           { return "password" }
func (passwordFactory) Capabilities() []string { return []string{"authenticator"} }
func (f passwordFactory) New() (Authenticator, ChallengeAuthenticator, DelegatedAuthenticator, error) {
	return f.p, nil, nil, nil
}

type webauthnFactory struct{ p *WebAuthnProvider }

// NewWebAuthnFactory wraps a provisioned WebAuthn (passkey) provider.
func NewWebAuthnFactory(p *WebAuthnProvider) Factory { return webauthnFactory{p} }

func (webauthnFactory) Name() string           { return "webauthn" }
func (webauthnFactory) Capabilities() []string { return []string{"challenge"} }
func (f webauthnFactory) New() (Authenticator, ChallengeAuthenticator, DelegatedAuthenticator, error) {
	return nil, f.p, nil, nil
}

type qrcodeFactory struct{ p *QRCodeProvider }

// NewQRCodeFactory wraps a provisioned QR (delegated) provider.
func NewQRCodeFactory(p *QRCodeProvider) Factory { return qrcodeFactory{p} }

func (qrcodeFactory) Name() string           { return "qrcode" }
func (qrcodeFactory) Capabilities() []string { return []string{"delegated"} }
func (f qrcodeFactory) New() (Authenticator, ChallengeAuthenticator, DelegatedAuthenticator, error) {
	return nil, nil, f.p, nil
}
