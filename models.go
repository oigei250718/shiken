package main

import (
	"html/template"
	"strings"
)

// Meaning 单词的一个含义及其例句
type Meaning struct {
	Text     string
	Examples []string
}

// WordRef 关联单词的简要信息
type WordRef struct {
	ID   int64
	Word string
	Kana string
}

// Word 单词完整信息
type Word struct {
	ID        int64
	Word      string
	Kana      string
	Level     int
	Meanings  []Meaning
	Related   []WordRef
	IsError   bool
	CreatedAt string
	UpdatedAt string
}

// MeaningTexts 返回所有含义文本
func (w *Word) MeaningTexts() []string {
	out := make([]string, 0, len(w.Meanings))
	for _, m := range w.Meanings {
		out = append(out, m.Text)
	}
	return out
}

// GrammarRef 关联语法的简要信息
type GrammarRef struct {
	ID     int64
	Format string
}

// Grammar 语法完整信息
type Grammar struct {
	ID        int64
	Format    string
	Level     int
	Meanings  []Meaning
	Related   []GrammarRef
	CreatedAt string
	UpdatedAt string
}

// Article 文章
type Article struct {
	ID        int64
	Title     string
	Content   string
	CreatedAt string
	UpdatedAt string
}

// Excerpt 内容摘要（列表页用），换行替换为空格，最多 80 字
func (a *Article) Excerpt() string {
	runes := []rune(strings.ReplaceAll(a.Content, "\n", " "))
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return string(runes)
}

// User 系统用户
type User struct {
	ID        int64
	Username  string
	Password  string // bcrypt 哈希
	Role      string // 'root' | 'admin' | 'user'
	Disabled  bool
	CreatedAt string
	UpdatedAt string
}

// IsRoot 是否为根用户
func (u *User) IsRoot() bool { return u != nil && u.Role == "root" }

// IsAdmin 是否为管理员或根用户
func (u *User) IsAdmin() bool { return u != nil && (u.Role == "root" || u.Role == "admin") }

// CanEdit 是否能录入/编辑/删除单词·语法·文章（root 与 admin）
func (u *User) CanEdit() bool { return u != nil && (u.Role == "root" || u.Role == "admin") }

// Page 分页信息
type Page struct {
	Current   int
	Total     int // 总页数
	Count     int // 总条数
	Query     string
	Extra     template.URL // 附加查询参数（内部生成，可信），如 "&level=3"
	HasPrev   bool
	HasNext   bool
}
