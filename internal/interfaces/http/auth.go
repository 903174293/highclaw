package http

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const webSessionCookieName = "hc_web_session"

func (s *Server) requiresWebLogin() bool {
	if s.cfg == nil {
		return false
	}
	auth := s.cfg.Web.Auth
	return auth.Enabled && strings.TrimSpace(auth.Username) != "" && strings.TrimSpace(auth.Password) != ""
}

func (s *Server) webSessionTTL() time.Duration {
	ttlMinutes := s.cfg.Web.Auth.SessionTTLMinutes
	if ttlMinutes <= 0 {
		ttlMinutes = 1440
	}
	return time.Duration(ttlMinutes) * time.Minute
}

func (s *Server) createWebSession(username string) (string, time.Time, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	expiresAt := time.Now().Add(s.webSessionTTL())

	s.webSessionMu.Lock()
	defer s.webSessionMu.Unlock()
	s.cleanupExpiredWebSessionsLocked()
	s.webSessions[token] = webSession{
		Username:  username,
		ExpiresAt: expiresAt,
	}
	return token, expiresAt, nil
}

func (s *Server) cleanupExpiredWebSessionsLocked() {
	now := time.Now()
	for token, sess := range s.webSessions {
		if now.After(sess.ExpiresAt) {
			delete(s.webSessions, token)
		}
	}
}

func (s *Server) validateWebSession(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	s.webSessionMu.Lock()
	defer s.webSessionMu.Unlock()
	s.cleanupExpiredWebSessionsLocked()
	sess, ok := s.webSessions[token]
	if !ok || time.Now().After(sess.ExpiresAt) {
		delete(s.webSessions, token)
		return "", false
	}
	return sess.Username, true
}

func (s *Server) removeWebSession(token string) {
	if token == "" {
		return
	}
	s.webSessionMu.Lock()
	defer s.webSessionMu.Unlock()
	delete(s.webSessions, token)
}

func (s *Server) getWebSessionToken(c *gin.Context) string {
	cookie, err := c.Cookie(webSessionCookieName)
	if err == nil && strings.TrimSpace(cookie) != "" {
		return strings.TrimSpace(cookie)
	}
	// Optional fallback for non-browser callers.
	if headerToken := strings.TrimSpace(c.GetHeader("X-Web-Session")); headerToken != "" {
		return headerToken
	}
	return ""
}

func (s *Server) setWebSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	secure := c.Request.TLS != nil
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
}

func (s *Server) clearWebSessionCookie(c *gin.Context) {
	secure := c.Request.TLS != nil
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func secureEquals(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) handleLogin(c *gin.Context) {
	if !s.requiresWebLogin() {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"username":      "anonymous",
			"loginEnabled":  false,
		})
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password
	cfgAuth := s.cfg.Web.Auth

	if !secureEquals(username, cfgAuth.Username) || !secureEquals(password, cfgAuth.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, expiresAt, err := s.createWebSession(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create login session"})
		return
	}
	s.setWebSessionCookie(c, token, expiresAt)
	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      username,
		"expiresAt":     expiresAt.UnixMilli(),
		"loginEnabled":  true,
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	// Browser BasicAuth credentials are managed by the browser.
	c.JSON(http.StatusOK, gin.H{
		"loggedOut": true,
		"message":   "basic auth is browser-managed; close browser or clear stored credentials to log out",
	})
}

func (s *Server) handleAuthMe(c *gin.Context) {
	if !s.requiresWebLogin() {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"username":      "anonymous",
			"loginEnabled":  false,
		})
		return
	}

	if s.validateBasicAuth(c) {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": true,
			"username":      strings.TrimSpace(s.cfg.Web.Auth.Username),
			"loginEnabled":  true,
			"authMode":      "basic",
		})
		return
	}
	s.requestBasicAuth(c)
}
