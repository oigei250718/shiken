package main

import (
	"net/http"
	"strings"
)

// defaultPassword 新用户与重置时的默认密码
const defaultPassword = "ppy0910."

// handleLoginGet 展示登录页
func handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if userFromContext(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	render(w, r, "login.html", map[string]any{})
}

// handleLoginPost 校验用户名密码，成功签发 JWT 并写入 cookie
func handleLoginPost(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	u, err := getUserByUsername(username)
	if err != nil || u.Disabled || !checkPassword(password, u.Password) {
		render(w, r, "login.html", map[string]any{"Error": "用户名或密码错误", "Username": username})
		return
	}
	token, err := generateToken(u)
	if err != nil {
		render(w, r, "login.html", map[string]any{"Error": "登录失败，请重试", "Username": username})
		return
	}
	setAuthCookie(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout 清除 cookie 并返回登录页
func handleLogout(w http.ResponseWriter, r *http.Request) {
	clearAuthCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleUserList 用户管理：列表
func handleUserList(w http.ResponseWriter, r *http.Request) {
	users, err := listUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, "user_list.html", map[string]any{"Users": users})
}

// handleUserNew 用户管理：新增用户表单
func handleUserNew(w http.ResponseWriter, r *http.Request) {
	render(w, r, "user_form.html", map[string]any{"Title": "新增用户"})
}

// handleUserCreate 用户管理：创建用户（密码为默认密码）
func handleUserCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username := strings.TrimSpace(r.FormValue("username"))
	role := r.FormValue("role")
	if username == "" {
		render(w, r, "user_form.html", map[string]any{"Title": "新增用户", "Error": "用户名不能为空"})
		return
	}
	switch role {
	case "admin", "user":
	default:
		role = "user"
	}
	if _, err := createUser(username, defaultPassword, role); err != nil {
		render(w, r, "user_form.html", map[string]any{
			"Title": "新增用户", "Error": "创建失败：" + err.Error(), "Username": username,
		})
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// handleUserDelete 用户管理：删除用户（root 自身不可删）
func handleUserDelete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	u, err := getUser(id)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusNotFound)
		return
	}
	if u.IsRoot() {
		http.Error(w, "不能删除 root 用户", http.StatusForbidden)
		return
	}
	if err := deleteUser(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// handleUserRole 用户管理：修改角色（root 不可改）
func handleUserRole(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	id := parseID(r, "id")
	role := r.FormValue("role")
	if role != "root" && role != "admin" && role != "user" {
		http.Error(w, "非法角色", http.StatusBadRequest)
		return
	}
	u, err := getUser(id)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusNotFound)
		return
	}
	if u.IsRoot() {
		http.Error(w, "不能修改 root 用户", http.StatusForbidden)
		return
	}
	if err := updateUserRole(id, role); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// handleUserToggle 用户管理：启用 / 禁用（root 自身不可禁用）
func handleUserToggle(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	id := parseID(r, "id")
	disabled := r.FormValue("disabled") == "1"
	u, err := getUser(id)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusNotFound)
		return
	}
	if u.IsRoot() {
		http.Error(w, "不能禁用 root 用户", http.StatusForbidden)
		return
	}
	if err := setUserDisabled(id, disabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// handleUserReset 用户管理：重置密码为默认密码（root 自身除外）
func handleUserReset(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	u, err := getUser(id)
	if err != nil {
		http.Error(w, "用户不存在", http.StatusNotFound)
		return
	}
	if u.IsRoot() {
		http.Error(w, "不能重置 root 用户密码", http.StatusForbidden)
		return
	}
	if err := resetUserPassword(id, defaultPassword); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/users?saved=pwd", http.StatusSeeOther)
}
