package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ===================== 单词测试 =====================

func handleTestSetup(w http.ResponseWriter, r *http.Request) {
	// 统计各级别单词数量，辅助用户选择
	counts := map[int]int{}
	rows, err := db.Query(`SELECT level, COUNT(*) FROM words GROUP BY level`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var lv, c int
			if rows.Scan(&lv, &c) == nil {
				counts[lv] = c
			}
		}
	}
	render(w, "test_setup.html", map[string]any{"Counts": counts})
}

type testMeaningJSON struct {
	Text     string   `json:"text"`
	Examples []string `json:"examples"`
}

type testWordJSON struct {
	ID       int64            `json:"id"`
	Word     string           `json:"word"`
	Kana     string           `json:"kana"`
	Meanings []testMeaningJSON `json:"meanings"`
}

func handleTestRun(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var levels []int
	for _, s := range q["level"] {
		for _, part := range splitComma(s) {
			if lv, err := strconv.Atoi(part); err == nil && lv >= 1 && lv <= 5 {
				levels = append(levels, lv)
			}
		}
	}
	if len(levels) == 0 {
		levels = []int{1, 2, 3, 4, 5}
	}
	mode := q.Get("mode")
	if mode != "word" && mode != "kana" && mode != "meaning" {
		mode = "word"
	}
	words, err := randomWordsForTest(levels, 20)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(words) == 0 {
		render(w, "test_run.html", map[string]any{"Empty": true, "Mode": mode})
		return
	}
	payload := make([]testWordJSON, 0, len(words))
	for _, wd := range words {
		ms := make([]testMeaningJSON, 0, len(wd.Meanings))
		for _, m := range wd.Meanings {
			ex := m.Examples
			if ex == nil {
				ex = []string{}
			}
			ms = append(ms, testMeaningJSON{Text: m.Text, Examples: ex})
		}
		payload = append(payload, testWordJSON{
			ID: wd.ID, Word: wd.Word, Kana: wd.Kana, Meanings: ms,
		})
	}
	data, _ := json.Marshal(payload)
	render(w, "test_run.html", map[string]any{
		"WordsJSON": string(data), "Mode": mode, "Total": len(payload),
	})
}

// handleTestFinish 接收测试结束时不认识的单词 ID，加入易错库
func handleTestFinish(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Total   int     `json:"total"`
		Unknown []int64 `json:"unknown"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "请求格式错误", http.StatusBadRequest)
		return
	}
	for _, id := range body.Unknown {
		_ = addErrorWord(id)
	}
	ids := make([]string, 0, len(body.Unknown))
	for _, id := range body.Unknown {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	redirect := "/test/result?total=" + strconv.Itoa(body.Total) +
		"&unknown=" + strconv.Itoa(len(body.Unknown))
	if len(ids) > 0 {
		redirect += "&ids=" + strings.Join(ids, ",")
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"redirect": redirect})
}

func handleTestResult(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	total, _ := strconv.Atoi(q.Get("total"))
	unknown, _ := strconv.Atoi(q.Get("unknown"))
	words, _ := getWordsByIDs(parseIDList(q.Get("ids")))
	render(w, "test_result.html", map[string]any{
		"Total": total, "Unknown": unknown, "Known": total - unknown,
		"UnknownWords": words,
	})
}

// ===================== 易错单词 =====================

func handleErrorWordList(w http.ResponseWriter, r *http.Request) {
	page := parsePage(r)
	items, total, err := listErrorWords(page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "error_list.html", map[string]any{
		"Items": items, "Page": makePage(page, total, ""),
	})
}

func handleErrorWordAddPage(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var results []Word
	if q != "" {
		var err error
		results, err = searchWordsNotInError(q)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	render(w, "error_add.html", map[string]any{"Query": q, "Results": results})
}

func handleErrorWordAdd(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("word_id"), 10, 64)
	if err := addErrorWord(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ref := r.FormValue("ref")
	if ref == "" {
		ref = "/error-words"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func handleErrorWordDelete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if err := deleteErrorWord(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ref := r.FormValue("ref")
	if ref == "" {
		ref = "/error-words"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}
