package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func handleGrammarList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := parsePage(r)
	items, total, err := listGrammars(q, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, "grammar_list.html", map[string]any{
		"Items": items, "Page": makePage(page, total, q),
	})
}

func handleGrammarNew(w http.ResponseWriter, r *http.Request) {
	render(w, "grammar_form.html", map[string]any{
		"Title": "录入语法", "Action": "/grammars",
		"Grammar": Grammar{Level: 3, Meanings: []Meaning{{}}},
	})
}

func grammarFromForm(r *http.Request) Grammar {
	g := Grammar{
		Format: strings.TrimSpace(r.FormValue("format")),
	}
	g.Level, _ = strconv.Atoi(r.FormValue("level"))
	if g.Level < 1 || g.Level > 5 {
		g.Level = 3
	}
	texts := r.Form["meaning_texts[]"]
	examples := r.Form["meaning_examples[]"]
	for i, t := range texts {
		m := Meaning{Text: t}
		if i < len(examples) {
			for _, line := range strings.Split(examples[i], "\n") {
				m.Examples = append(m.Examples, strings.TrimSpace(line))
			}
		}
		g.Meanings = append(g.Meanings, m)
	}
	return g
}

func handleGrammarCreate(w http.ResponseWriter, r *http.Request) {
	g := grammarFromForm(r)
	if g.Format == "" {
		http.Error(w, "语法格式不能为空", http.StatusBadRequest)
		return
	}
	id, err := createGrammar(&g)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, rid := range relatedGrammarIDsFromForm(r) {
		_ = addGrammarRelation(id, rid)
	}
	http.Redirect(w, r, "/grammars/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func handleGrammarDetail(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	g, err := getGrammar(id)
	if err != nil {
		http.Error(w, "语法不存在", http.StatusNotFound)
		return
	}
	prev, next := grammarPrevNext(id)
	render(w, "grammar_detail.html", map[string]any{
		"Grammar": g, "Prev": prev, "Next": next,
		"Back": r.URL.Query().Get("back"),
	})
}

func handleGrammarEdit(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	g, err := getGrammar(id)
	if err != nil {
		http.Error(w, "语法不存在", http.StatusNotFound)
		return
	}
	if len(g.Meanings) == 0 {
		g.Meanings = []Meaning{{}}
	}
	render(w, "grammar_form.html", map[string]any{
		"Title": "编辑语法", "Action": "/grammars/" + strconv.FormatInt(id, 10),
		"Grammar": g,
	})
}

func handleGrammarUpdate(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	g := grammarFromForm(r)
	g.ID = id
	if g.Format == "" {
		http.Error(w, "语法格式不能为空", http.StatusBadRequest)
		return
	}
	if err := updateGrammar(&g); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	syncGrammarRelations(id, relatedGrammarIDsFromForm(r))
	http.Redirect(w, r, "/grammars/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// relatedGrammarIDsFromForm 解析表单中选中的关联语法 ID 列表
func relatedGrammarIDsFromForm(r *http.Request) []int64 {
	var out []int64
	for _, s := range r.Form["related_ids[]"] {
		out = append(out, parseIDList(s)...)
	}
	return out
}

// syncGrammarRelations 将语法的关联关系同步为目标集合（新增缺失的，删除多余的）
func syncGrammarRelations(id int64, target []int64) {
	cur, err := getGrammar(id)
	if err != nil {
		return
	}
	want := map[int64]bool{}
	for _, rid := range target {
		want[rid] = true
	}
	have := map[int64]bool{}
	for _, ref := range cur.Related {
		have[ref.ID] = true
	}
	for rid := range want {
		if !have[rid] {
			_ = addGrammarRelation(id, rid)
		}
	}
	for rid := range have {
		if !want[rid] {
			_ = deleteGrammarRelation(id, rid)
		}
	}
}

func handleGrammarDelete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	if err := deleteGrammars([]int64{id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/grammars", http.StatusSeeOther)
}

func handleGrammarBatchDelete(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	var ids []int64
	for _, s := range r.Form["ids"] {
		ids = append(ids, parseIDList(s)...)
	}
	if err := deleteGrammars(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/grammars", http.StatusSeeOther)
}

func handleGrammarRelatePage(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	g, err := getGrammar(id)
	if err != nil {
		http.Error(w, "语法不存在", http.StatusNotFound)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var results []Grammar
	if q != "" {
		all, _, err := listGrammars(q, 1)
		if err == nil {
			// 排除自身与已关联的
			related := map[int64]bool{id: true}
			for _, ref := range g.Related {
				related[ref.ID] = true
			}
			for _, item := range all {
				if !related[item.ID] {
					results = append(results, item)
				}
			}
		}
	}
	render(w, "grammar_relate.html", map[string]any{
		"Grammar": g, "Query": q, "Results": results,
	})
}

func handleGrammarRelateAdd(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	rid, _ := strconv.ParseInt(r.FormValue("rid"), 10, 64)
	if err := addGrammarRelation(id, rid); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ref := r.FormValue("ref")
	if ref == "" {
		ref = "/grammars/" + strconv.FormatInt(id, 10)
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func handleGrammarRelateDelete(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	rid := parseID(r, "rid")
	if err := deleteGrammarRelation(id, rid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/grammars/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// handleGrammarReview 随机抽取 20 条（按级别分布等比例）
func handleGrammarReview(w http.ResponseWriter, r *http.Request) {
	var items []Grammar
	idsParam := r.URL.Query().Get("ids")
	if idsParam != "" {
		for _, id := range parseIDList(idsParam) {
			if g, err := getGrammar(id); err == nil {
				items = append(items, *g)
			}
		}
	} else {
		var err error
		items, err = sampleGrammarsForReview(20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// 重定向到带 ids 的 URL，保证详情页返回时列表不变
		ids := make([]string, 0, len(items))
		for _, g := range items {
			ids = append(ids, strconv.FormatInt(g.ID, 10))
		}
		if len(ids) > 0 {
			http.Redirect(w, r, "/grammars/review?ids="+strings.Join(ids, ","), http.StatusSeeOther)
			return
		}
	}
	back := "/grammars/review?ids=" + idsParam
	render(w, "grammar_review.html", map[string]any{
		"Items": items, "Back": url.QueryEscape(back),
	})
}
