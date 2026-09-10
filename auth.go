package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const cookieToken = "shiken_token"
const tokenTTL = 14 * 24 * time.Hour

// jwtSecret 从环境变量读取，缺失时使用默认值（生产应覆盖）
func jwtSecret() []byte {
	s := os.Getenv("SHIKEN_JWT_SECRET")
	if s == "" {
		return []byte("shiken-dev-secret-change-me")
	}
	return []byte(s)
}

type ctxKeyUser struct{}

// userFromContext 取出当前登录用户（未登录返回 nil）
func userFromContext(r *http.Request) *User {
	if u, ok := r.Context().Value(ctxKeyUser{}).(*User); ok {
		return u
	}
	return nil
}

func withUser(r *http.Request, u *User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKeyUser{}, u))
}

// currentUserID 当前登录用户 ID（未登录为 0）
func currentUserID(r *http.Request) int64 {
	if u := userFromContext(r); u != nil {
		return u.ID
	}
	return 0
}

// hashPassword 使用 bcrypt 生成密码哈希
func hashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// checkPassword 校验明文与 bcrypt 哈希
func checkPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// generateToken 签发 JWT（载荷含 uid / role / username，有效期 14 天）
func generateToken(u User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"uid":  u.ID,
		"role": u.Role,
		"usr":  u.Username,
		"iat":  now.Unix(),
		"exp":  now.Add(tokenTTL).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret())
}

// parseTokenClaims 解析并校验 JWT
func parseTokenClaims(token string) (uid int64, role, username string, err error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return 0, "", "", err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return 0, "", "", jwt.ErrTokenInvalidClaims
	}
	uidF, _ := claims["uid"].(float64)
	uid = int64(uidF)
	role, _ = claims["role"].(string)
	username, _ = claims["usr"].(string)
	return uid, role, username, nil
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieToken,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(tokenTTL.Seconds()),
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieToken,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func isAPI(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/api/")
}

// authUser 从 cookie 解析并加载最新用户；token 无效或用户被禁用时返回 nil 并清除 cookie
func authUser(w http.ResponseWriter, r *http.Request) *User {
	c, err := r.Cookie(cookieToken)
	if err != nil || c.Value == "" {
		return nil
	}
	uid, _, _, err := parseTokenClaims(c.Value)
	if err != nil {
		clearAuthCookie(w)
		return nil
	}
	u, err := getUser(uid)
	if err != nil {
		clearAuthCookie(w)
		return nil
	}
	if u.Disabled {
		clearAuthCookie(w)
		return nil
	}
	return &u
}

// requireAuth 登录中间件：未登录跳 /login（API 请求返回 401 JSON）
func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := authUser(w, r)
		if u == nil {
			if isAPI(r) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "未登录或登录已失效"})
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, withUser(r, u))
	}
}

// requireRoot 仅 root 可访问
func requireRoot(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(func(w http.ResponseWriter, r *http.Request) {
		u := userFromContext(r)
		if u == nil || !u.IsRoot() {
			http.Error(w, "需要 root 权限", http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// requireEditor 仅 root / admin 可操作；不满足时返回 false 并已写入 403
func requireEditor(w http.ResponseWriter, r *http.Request) bool {
	u := userFromContext(r)
	if u == nil || !u.CanEdit() {
		http.Error(w, "无权限：仅管理员可操作", http.StatusForbidden)
		return false
	}
	return true
}
