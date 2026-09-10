package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func handleWordList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	level, _ := strconv.Atoi(r.URL.Query().Get("level"))
	page := parsePage(r)
	items, total, err := listWords(q, page, level, currentUserID(r))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	extra := ""
	if level >= 1 && level <= 5 {
		extra = "&level=" + strconv.Itoa(level)
	}
	render(w, r, "word_list.html", map[string]any{
		"Items": items, "Page": makePage(page, total, q, extra), "Level": level,
	})
}

func handleWordNew(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	render(w, r, "word_form.html", map[string]any{
		"Title": "录入单词", "Action": "/words",
		"Word":    Word{Level: 3, Meanings: []Meaning{{}}},
		"Saved":   r.URL.Query().Get("saved"),
		"SavedID": r.URL.Query().Get("id"),
	})
}

func wordFromForm(r *http.Request) Word {
	w := Word{
		Word: strings.TrimSpace(r.FormValue("word")),
		Kana: strings.TrimSpace(r.FormValue("kana")),
	}
	w.Level, _ = strconv.Atoi(r.FormValue("level"))
	if w.Level < 1 || w.Level > 5 {
		w.Level = 3
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
		w.Meanings = append(w.Meanings, m)
	}
	return w
}

func handleWordCreate(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	wd := wordFromForm(r)
	if wd.Word == "" {
		http.Error(w, "单词不能为空", http.StatusBadRequest)
		return
	}
	id, err := createWord(&wd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, rid := range relatedIDsFromForm(r) {
		_ = addWordRelation(id, rid)
	}
	// 保存后回到录入页，方便连续录入；附带去详情页的链接
	http.Redirect(w, r, "/words/new?saved="+url.QueryEscape(wd.Word)+"&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func handleWordDetail(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	wd, err := getWord(id, currentUserID(r))
	if err != nil {
		http.Error(w, "单词不存在", http.StatusNotFound)
		return
	}
	prev, next := wordPrevNext(id)
	render(w, r, "word_detail.html", map[string]any{
		"Word": wd, "Prev": prev, "Next": next,
		"Back": r.URL.Query().Get("back"),
	})
}

func handleWordEdit(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	wd, err := getWord(id, currentUserID(r))
	if err != nil {
		http.Error(w, "单词不存在", http.StatusNotFound)
		return
	}
	if len(wd.Meanings) == 0 {
		wd.Meanings = []Meaning{{}}
	}
	render(w, r, "word_form.html", map[string]any{
		"Title": "编辑单词", "Action": "/words/" + strconv.FormatInt(id, 10),
		"Word": wd,
	})
}

func handleWordUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	wd := wordFromForm(r)
	wd.ID = id
	if wd.Word == "" {
		http.Error(w, "单词不能为空", http.StatusBadRequest)
		return
	}
	if err := updateWord(&wd); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	syncWordRelations(id, relatedIDsFromForm(r))
	http.Redirect(w, r, "/words/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// relatedIDsFromForm 解析表单中选中的关联单词 ID 列表
func relatedIDsFromForm(r *http.Request) []int64 {
	var out []int64
	for _, s := range r.Form["related_ids[]"] {
		out = append(out, parseIDList(s)...)
	}
	return out
}

// syncWordRelations 将单词的关联关系同步为目标集合（新增缺失的，删除多余的）
func syncWordRelations(id int64, target []int64) {
	cur, err := getWord(id, 0)
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
			_ = addWordRelation(id, rid)
		}
	}
	for rid := range have {
		if !want[rid] {
			_ = deleteWordRelation(id, rid)
		}
	}
}

func handleWordDelete(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	if err := deleteWords([]int64{id}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/words", http.StatusSeeOther)
}

func handleWordBatchDelete(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	_ = r.ParseForm()
	var ids []int64
	for _, s := range r.Form["ids"] {
		ids = append(ids, parseIDList(s)...)
	}
	if err := deleteWords(ids); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/words", http.StatusSeeOther)
}

func handleWordRelatePage(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	wd, err := getWord(id, currentUserID(r))
	if err != nil {
		http.Error(w, "单词不存在", http.StatusNotFound)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	var results []Word
	if q != "" {
		all, _, err := listWords(q, 1, 0, currentUserID(r))
		if err == nil {
			related := map[int64]bool{id: true}
			for _, ref := range wd.Related {
				related[ref.ID] = true
			}
			for _, item := range all {
				if !related[item.ID] {
					results = append(results, item)
				}
			}
		}
	}
	render(w, r, "word_relate.html", map[string]any{
		"Word": wd, "Query": q, "Results": results,
	})
}

func handleWordRelateAdd(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	rid, _ := strconv.ParseInt(r.FormValue("rid"), 10, 64)
	if err := addWordRelation(id, rid); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ref := r.FormValue("ref")
	if ref == "" {
		ref = "/words/" + strconv.FormatInt(id, 10)
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

func handleWordRelateDelete(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	rid := parseID(r, "rid")
	if err := deleteWordRelation(id, rid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/words/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}
