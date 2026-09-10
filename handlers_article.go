package main

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func handleArticleList(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	page := parsePage(r)
	items, total, err := listArticles(q, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	render(w, r, "article_list.html", map[string]any{
		"Items": items, "Page": makePage(page, total, q),
	})
}

func handleArticleNew(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	render(w, r, "article_form.html", map[string]any{
		"Title": "录入文章", "Action": "/articles",
		"Article": Article{},
		"Saved":   r.URL.Query().Get("saved"),
		"SavedID": r.URL.Query().Get("id"),
	})
}

func handleArticleCreate(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	a := articleFromForm(r)
	if a.Title == "" {
		http.Error(w, "标题不能为空", http.StatusBadRequest)
		return
	}
	id, err := createArticle(&a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 保存后回到录入页，方便连续录入；附带去详情页的链接
	http.Redirect(w, r, "/articles/new?saved="+url.QueryEscape(a.Title)+"&id="+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func handleArticleDetail(w http.ResponseWriter, r *http.Request) {
	id := parseID(r, "id")
	a, err := getArticle(id)
	if err != nil {
		http.Error(w, "文章不存在", http.StatusNotFound)
		return
	}
	render(w, r, "article_detail.html", map[string]any{
		"Article": a,
	})
}

func handleArticleEdit(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	a, err := getArticle(id)
	if err != nil {
		http.Error(w, "文章不存在", http.StatusNotFound)
		return
	}
	render(w, r, "article_form.html", map[string]any{
		"Title": "编辑文章", "Action": "/articles/" + strconv.FormatInt(id, 10),
		"Article": a,
	})
}

func handleArticleUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	if _, err := getArticle(id); err != nil {
		http.Error(w, "文章不存在", http.StatusNotFound)
		return
	}
	a := articleFromForm(r)
	if a.Title == "" {
		http.Error(w, "标题不能为空", http.StatusBadRequest)
		return
	}
	a.ID = id
	if err := updateArticle(&a); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/articles/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func handleArticleDelete(w http.ResponseWriter, r *http.Request) {
	if !requireEditor(w, r) {
		return
	}
	id := parseID(r, "id")
	if err := deleteArticle(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/articles", http.StatusSeeOther)
}

func articleFromForm(r *http.Request) Article {
	return Article{
		Title:   strings.TrimSpace(r.FormValue("title")),
		Content: strings.TrimSpace(r.FormValue("content")),
	}
}
