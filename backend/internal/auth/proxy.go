package auth

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// NewProxy forwards /api/auth/* to Goauth's /auth/* so the browser only ever
// talks to one origin, which is what lets the SameSite=Strict refresh cookie work.
func NewProxy(goauthBaseURL string) (http.Handler, error) {
	target, err := url.Parse(goauthBaseURL)
	if err != nil {
		return nil, err
	}
	rp := httputil.NewSingleHostReverseProxy(target)

	orig := rp.Director
	rp.Director = func(r *http.Request) {
		orig(r)
		r.URL.Path = strings.Replace(r.URL.Path, "/api/auth", "/auth", 1)
		r.Host = target.Host
	}

	// Goauth scopes the refresh cookie to Path=/auth; the browser sees
	// /api/auth, so the path has to be rewritten or the cookie is never sent.
	rp.ModifyResponse = func(resp *http.Response) error {
		cookies := resp.Header["Set-Cookie"]
		for i, c := range cookies {
			cookies[i] = rewriteCookiePath(c, "/auth", "/api/auth")
		}
		return nil
	}
	return rp, nil
}

func rewriteCookiePath(cookie, from, to string) string {
	parts := strings.Split(cookie, ";")
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if len(trimmed) < 5 || !strings.EqualFold(trimmed[:5], "path=") {
			continue
		}
		path := strings.TrimSpace(trimmed[5:])
		if path == from || strings.HasPrefix(path, from+"/") {
			parts[i] = " Path=" + to + path[len(from):]
		}
	}
	return strings.Join(parts, ";")
}
