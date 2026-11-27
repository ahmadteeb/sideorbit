package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"html/template"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ahmadteeb/sideorbit/server/internal/httpx"
	"github.com/ahmadteeb/sideorbit/server/internal/store/db"
)

// normalizeEmail accepts a bare address only and lowercases it.
func normalizeEmail(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !ascii(s) {
		return "", false
	}
	s = strings.ToLower(s)
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || len(s) > 254 || strings.ContainsAny(s, "\r\n<>") {
		return "", false
	}
	return s, true
}

// maskEmail keeps logs free of full addresses.
func maskEmail(s string) string {
	local, domain, ok := strings.Cut(s, "@")
	if !ok || local == "" {
		return "***"
	}
	return local[:1] + "***@" + domain
}

// emailStart mails a sign-in link. It answers 202 whether or not an account
// exists, and never looks one up.
func (s *Service) emailStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email         string `json:"email"`
		Client        string `json:"client"`
		CodeChallenge string `json:"code_challenge"`
		Redirect      string `json:"redirect"`
	}
	if !httpx.Decode(w, r, &req) {
		return
	}
	email, ok := normalizeEmail(req.Email)
	if !ok {
		httpx.Error(w, http.StatusBadRequest, "invalid_email", "That doesn't look like an email address.", "Check for typos.")
		return
	}
	t, err := s.parseTarget(req.Client, req.CodeChallenge, req.Redirect)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_request", err.Error(), "")
		return
	}
	// The IP check comes first so one caller cannot use up another address's quota.
	if !s.mailByIP.Allow(httpx.ClientIP(r)) || !s.mailByAddress.Allow(email) || !s.mailTotal.Allow("") {
		s.log.Warn("rate limit hit", "endpoint", "email_start", "email", maskEmail(email))
		w.Header().Set("Retry-After", "900")
		httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "Too many sign-in emails.", "Wait 15 minutes, then try again.")
		return
	}
	token := randomToken()
	err = s.q.CreateLoginCode(r.Context(), db.CreateLoginCodeParams{
		CodeHash: hashToken(token), Purpose: purposeMagic, Email: &email,
		CodeChallenge: optional(t.challenge), Client: &t.client, Redirect: optional(t.redirect),
		TtlSeconds: int32(magicTTL / time.Second),
	})
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	link := s.cfg.PublicURL.JoinPath("api", "auth", "email", "verify")
	link.RawQuery = "token=" + token
	body := "Open this link to sign in to Orbit:\n\n" + link.String() +
		"\n\nIt works once and expires in 15 minutes.\nIf you didn't ask for it, ignore this email.\n"
	if err := s.cfg.Mailer.Send(r.Context(), email, "Sign in to Orbit", body); err != nil {
		s.log.Error("could not send sign-in email", "email", maskEmail(email), "err", strings.ReplaceAll(err.Error(), email, maskEmail(email)))
		httpx.Error(w, http.StatusServiceUnavailable, "email_unavailable", "Orbit couldn't send the email.", "Try again in a few minutes.")
		return
	}
	s.log.Info("sign-in email sent", "email", maskEmail(email))
	w.WriteHeader(http.StatusAccepted)
}

// The pages use browser defaults so they follow the system theme and text size.
const pageStyle = `body{font-family:system-ui,sans-serif;line-height:1.5;max-width:32rem;margin:0 auto;padding:2rem 1rem;overflow-wrap:anywhere}button{font:inherit;min-height:44px;padding:0 1.5rem;margin:0 .5rem .5rem 0}`

var pageCSP = func() string {
	sum := sha256.Sum256([]byte(pageStyle))
	return "default-src 'none'; style-src 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) +
		"'; frame-ancestors 'none'; base-uri 'none'"
}()

var page = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="light dark">
<title>Sign in to Orbit</title>
<style>` + pageStyle + `</style>
</head>
<body>
<main>
{{if .How}}<h1>Was this account yours?</h1>
<p>An Orbit account for <strong>{{.Email}}</strong> was already created with {{.How}} on {{.Since}}. You've just shown the address is yours now.</p>
<p>Did you create that account? If you're not sure, choose "No": someone who used this address before you could still sign in to it.</p>
<form method="post" action="/api/auth/claim">
<input type="hidden" name="token" value="{{.Token}}">
<button type="submit" name="answer" value="keep">Yes, keep it</button>
<button type="submit" name="answer" value="remove">No, remove everything in it</button>
</form>
<p>"No" deletes its devices, history, app sources and other sign-in methods. You'll then continue in the {{.App}} with an empty account.</p>{{else if .Token}}<h1>Sign in to Orbit</h1>
<p>Continue as <strong>{{.Email}}</strong> in the <strong>{{.App}}</strong>.</p>
<p>If that isn't your address, or you didn't just ask for this link there, close this page.</p>
<form method="post" action="{{.Action}}">
<input type="hidden" name="token" value="{{.Token}}">
<button type="submit">Sign in</button>
</form>{{else}}<h1>This link no longer works</h1>
<p>This sign-in has expired or was already used. Start again in Orbit.</p>{{end}}
</main>
</body>
</html>
`))

var appNames = map[string]string{ClientWeb: "web dashboard", ClientIOS: "Orbit iPhone app", ClientDesktop: "Orbit Setup desktop app"}

type pageData struct {
	Action, Token, Email, App string
	How, Since                string // set only for the "was this yours?" question
}

type claimPage struct{ Token, Email, App, How, Since string }

func writePage(w http.ResponseWriter, status int, action, token, email, client string) {
	render(w, status, pageData{Action: action, Token: token, Email: email, App: appNames[client]})
}

func writeClaimPage(w http.ResponseWriter, p claimPage) {
	render(w, http.StatusOK, pageData{Token: p.Token, Email: p.Email, App: p.App, How: p.How, Since: p.Since})
}

func render(w http.ResponseWriter, status int, data pageData) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", pageCSP)
	h.Set("Referrer-Policy", "same-origin") // the token is in the URL
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = page.Execute(w, data)
}

// emailConfirm only shows a button. Mail scanners open links, and a GET that
// consumed the token would sign nobody in.
func (s *Service) emailConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	code, err := s.q.PeekLoginCode(r.Context(), db.PeekLoginCodeParams{CodeHash: hashToken(token), Purpose: purposeMagic})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (code.Email == nil || code.Client == nil)) {
		writePage(w, http.StatusBadRequest, "", "", "", "")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	writePage(w, http.StatusOK, "/api/auth/email/verify", token, *code.Email, *code.Client)
}

func (s *Service) emailVerify(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	code, err := s.q.ConsumeLoginCode(r.Context(), db.ConsumeLoginCodeParams{CodeHash: hashToken(r.PostFormValue("token")), Purpose: purposeMagic})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (code.Email == nil || code.Client == nil)) {
		s.log.Warn("sign-in refused", "provider", "email", "reason", "link")
		writePage(w, http.StatusBadRequest, "", "", "", "")
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	id := identity{subject: *code.Email, email: *code.Email, ownsMailbox: true}
	user, err := s.findOrCreateUser(r.Context(), "email", id)
	t := target{client: *code.Client}
	if code.CodeChallenge != nil && code.Redirect != nil {
		t.challenge, t.redirect = *code.CodeChallenge, *code.Redirect
	}
	if errors.Is(err, errNeedsClaim) {
		s.askAboutAccount(w, r, user, "email", id, t)
		return
	}
	if err != nil {
		httpx.Internal(w, r, err)
		return
	}
	s.log.Info("identity verified", "provider", "email", "user_id", user.ID, "client", t.client)
	s.finish(w, r, user, t, http.StatusSeeOther)
}
