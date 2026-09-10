package main

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed templates static
var assets embed.FS

var tmpl *template.Template

// mdEngine Markdown 渲染器（含 GFM：表格 / 删除线 / 任务列表 / 自动链接）
var mdEngine = goldmark.New(goldmark.WithExtensions(extension.GFM))

// renderMarkdown 把 Markdown 文本渲染为 HTML（用于详情页展示）
func renderMarkdown(s string) template.HTML {
	var buf bytes.Buffer
	if err := mdEngine.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(buf.String())
}

var funcMap = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"mul": func(a, b int) int { return a * b },
	"markdown": renderMarkdown,
	"minute": func(s string) string { // 时间精确到分钟
		if len(s) > 16 {
			return s[:16]
		}
		return s
	},
	"iterate": func(n int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = i + 1
		}
		return out
	},
	"levelBadge": func(l int) string {
		labels := map[int]string{1: "最简单", 2: "简单", 3: "中等", 4: "困难", 5: "最困难"}
		if s, ok := labels[l]; ok {
			return s
		}
		return ""
	},
	"roleLabel": func(role string) string {
		switch role {
		case "root":
			return "根用户"
		case "admin":
			return "管理员"
		case "user":
			return "普通用户"
		}
		return role
	},
}

func main() {
	//backupDB()
	dsn := os.Getenv("SHIKEN_MYSQL_DSN")
	if dsn == "" {
		dsn = mysqlDSN
	}
	if err := openDB(dsn); err != nil {
		log.Fatalf("打开 MySQL 数据库失败: %v", err)
	}
	defer db.Close()

	tmpl = template.Must(template.New("").Funcs(funcMap).ParseFS(assets, "templates/*.html"))

	mux := http.NewServeMux()

	// 公开路由：登录 / 静态资源
	mux.HandleFunc("GET /login", handleLoginGet)
	mux.HandleFunc("POST /login", handleLoginPost)
	mux.HandleFunc("POST /logout", handleLogout)
	mux.Handle("GET /static/", http.FileServerFS(assets))

	// 用户管理（仅 root）
	mux.HandleFunc("GET /users", requireRoot(handleUserList))
	mux.HandleFunc("GET /users/new", requireRoot(handleUserNew))
	mux.HandleFunc("POST /users", requireRoot(handleUserCreate))
	mux.HandleFunc("POST /users/{id}/delete", requireRoot(handleUserDelete))
	mux.HandleFunc("POST /users/{id}/role", requireRoot(handleUserRole))
	mux.HandleFunc("POST /users/{id}/toggle", requireRoot(handleUserToggle))
	mux.HandleFunc("POST /users/{id}/reset", requireRoot(handleUserReset))

	// 业务路由：全部需登录
	// 首页
	mux.HandleFunc("GET /{$}", requireAuth(handleHome))

	// 语法
	mux.HandleFunc("GET /grammars", requireAuth(handleGrammarList))
	mux.HandleFunc("GET /grammars/new", requireAuth(handleGrammarNew))
	mux.HandleFunc("POST /grammars", requireAuth(handleGrammarCreate))
	mux.HandleFunc("POST /grammars/delete", requireAuth(handleGrammarBatchDelete))
	mux.HandleFunc("GET /grammars/review", requireAuth(handleGrammarReview))
	mux.HandleFunc("GET /grammars/{id}", requireAuth(handleGrammarDetail))
	mux.HandleFunc("GET /grammars/{id}/edit", requireAuth(handleGrammarEdit))
	mux.HandleFunc("POST /grammars/{id}", requireAuth(handleGrammarUpdate))
	mux.HandleFunc("POST /grammars/{id}/delete", requireAuth(handleGrammarDelete))
	mux.HandleFunc("GET /grammars/{id}/relate", requireAuth(handleGrammarRelatePage))
	mux.HandleFunc("POST /grammars/{id}/relations", requireAuth(handleGrammarRelateAdd))
	mux.HandleFunc("POST /grammars/{id}/relations/{rid}/delete", requireAuth(handleGrammarRelateDelete))

	// 文章
	mux.HandleFunc("GET /articles", requireAuth(handleArticleList))
	mux.HandleFunc("GET /articles/new", requireAuth(handleArticleNew))
	mux.HandleFunc("POST /articles", requireAuth(handleArticleCreate))
	mux.HandleFunc("GET /articles/{id}", requireAuth(handleArticleDetail))
	mux.HandleFunc("GET /articles/{id}/edit", requireAuth(handleArticleEdit))
	mux.HandleFunc("POST /articles/{id}", requireAuth(handleArticleUpdate))
	mux.HandleFunc("POST /articles/{id}/delete", requireAuth(handleArticleDelete))

	// 单词
	mux.HandleFunc("GET /words", requireAuth(handleWordList))
	mux.HandleFunc("GET /words/new", requireAuth(handleWordNew))
	mux.HandleFunc("POST /words", requireAuth(handleWordCreate))
	mux.HandleFunc("POST /words/delete", requireAuth(handleWordBatchDelete))
	mux.HandleFunc("GET /words/{id}", requireAuth(handleWordDetail))
	mux.HandleFunc("GET /words/{id}/edit", requireAuth(handleWordEdit))
	mux.HandleFunc("POST /words/{id}", requireAuth(handleWordUpdate))
	mux.HandleFunc("POST /words/{id}/delete", requireAuth(handleWordDelete))
	mux.HandleFunc("GET /words/{id}/relate", requireAuth(handleWordRelatePage))
	mux.HandleFunc("POST /words/{id}/relations", requireAuth(handleWordRelateAdd))
	mux.HandleFunc("POST /words/{id}/relations/{rid}/delete", requireAuth(handleWordRelateDelete))

	// 单词测试
	mux.HandleFunc("GET /test", requireAuth(handleTestSetup))
	mux.HandleFunc("GET /test/run", requireAuth(handleTestRun))
	mux.HandleFunc("POST /test/finish", requireAuth(handleTestFinish))
	mux.HandleFunc("GET /test/result", requireAuth(handleTestResult))

	// 易错单词
	mux.HandleFunc("GET /error-words", requireAuth(handleErrorWordList))
	mux.HandleFunc("GET /error-words/add", requireAuth(handleErrorWordAddPage))
	mux.HandleFunc("POST /error-words/add", requireAuth(handleErrorWordAdd))
	mux.HandleFunc("POST /error-words/{id}/delete", requireAuth(handleErrorWordDelete))

	// API
	mux.HandleFunc("GET /api/words/search", requireAuth(handleWordSearchAPI))
	mux.HandleFunc("GET /api/grammars/search", requireAuth(handleGrammarSearchAPI))

	log.Println("服务已启动: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
