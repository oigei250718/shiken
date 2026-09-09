package main

import (
	"html/template"
	"log"
	"net/http"
	"strconv"
)

func render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("渲染模板 %s 失败: %v", name, err)
		http.Error(w, "页面渲染失败", http.StatusInternalServerError)
	}
}

func parsePage(r *http.Request) int {
	p, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if p < 1 {
		p = 1
	}
	return p
}

func parseID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}

func totalPages(count int) int {
	if count == 0 {
		return 1
	}
	return (count + pageSize - 1) / pageSize
}

func makePage(current, count int, query string, extra ...string) Page {
	tp := totalPages(count)
	if current > tp {
		current = tp
	}
	p := Page{
		Current: current, Total: tp, Count: count, Query: query,
		HasPrev: current > 1, HasNext: current < tp,
	}
	if len(extra) > 0 {
		p.Extra = template.URL(extra[0])
	}
	return p
}

// parseIDList 解析形如 "1,2,3" 或多个同名表单值的 ID 列表
func parseIDList(s string) []int64 {
	var out []int64
	for _, part := range splitComma(s) {
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if part := s[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	var wordCount, grammarCount, errorCount, articleCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&wordCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM grammars`).Scan(&grammarCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM error_words`).Scan(&errorCount)
	_ = db.QueryRow(`SELECT COUNT(*) FROM articles`).Scan(&articleCount)
	render(w, "home.html", map[string]any{
		"WordCount": wordCount, "GrammarCount": grammarCount, "ErrorCount": errorCount,
		"ArticleCount": articleCount,
	})
}
