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
		cookies := resp.Cookies()
		resp.Header.Del("Set-Cookie")
		for _, c := range cookies {
			if c.Path == "/auth" || strings.HasPrefix(c.Path, "/auth/") {
				c.Path = "/api" + c.Path
			}
			resp.Header.Add("Set-Cookie", c.String())
		}
		return nil
	}
	return rp, nil
}
