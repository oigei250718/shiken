package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type wordSearchResult struct {
	ID       int64    `json:"id"`
	Word     string   `json:"word"`
	Kana     string   `json:"kana"`
	Meanings []string `json:"meanings"`
}

// handleWordSearchAPI 模糊搜索单词（用于表单内选择关联单词）
// GET /api/words/search?q=xxx&exclude=123
func handleWordSearchAPI(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	exclude, _ := strconv.ParseInt(r.URL.Query().Get("exclude"), 10, 64)
	results := []wordSearchResult{}
	if q != "" {
		like := "%" + q + "%"
		rows, err := db.Query(`SELECT w.id, w.word, w.kana FROM words w
			WHERE w.id != ? AND (w.word LIKE ? OR w.kana LIKE ?
			  OR EXISTS (SELECT 1 FROM word_meanings m WHERE m.word_id = w.id AND m.meaning LIKE ?))
			ORDER BY w.created_at DESC, w.id DESC LIMIT 20`, exclude, like, like, like)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var item wordSearchResult
			if err := rows.Scan(&item.ID, &item.Word, &item.Kana); err != nil {
				continue
			}
			if ms, err := wordMeanings(item.ID); err == nil {
				for _, m := range ms {
					item.Meanings = append(item.Meanings, m.Text)
				}
			}
			results = append(results, item)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

type grammarSearchResult struct {
	ID       int64    `json:"id"`
	Format   string   `json:"format"`
	Meanings []string `json:"meanings"`
}

// handleGrammarSearchAPI 模糊搜索语法（用于表单内选择关联语法）
// GET /api/grammars/search?q=xxx&exclude=123
func handleGrammarSearchAPI(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	exclude, _ := strconv.ParseInt(r.URL.Query().Get("exclude"), 10, 64)
	results := []grammarSearchResult{}
	if q != "" {
		like := "%" + q + "%"
		rows, err := db.Query(`SELECT g.id, g.format FROM grammars g
			WHERE g.id != ? AND (g.format LIKE ?
			  OR EXISTS (SELECT 1 FROM grammar_meanings m WHERE m.grammar_id = g.id AND m.meaning LIKE ?)
			  OR EXISTS (SELECT 1 FROM grammar_meaning_examples e
			              JOIN grammar_meanings m ON m.id = e.meaning_id
			              WHERE m.grammar_id = g.id AND e.example LIKE ?))
			ORDER BY g.created_at DESC, g.id DESC LIMIT 20`, exclude, like, like, like)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var item grammarSearchResult
			if err := rows.Scan(&item.ID, &item.Format); err != nil {
				continue
			}
			if ms, err := grammarMeanings(item.ID); err == nil {
				for _, m := range ms {
					if m.Text != "" {
						item.Meanings = append(item.Meanings, m.Text)
					}
				}
			}
			results = append(results, item)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}
