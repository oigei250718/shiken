package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
)

//go:embed templates static
var assets embed.FS

var tmpl *template.Template

var funcMap = template.FuncMap{
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"mul": func(a, b int) int { return a * b },
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

	// 首页
	mux.HandleFunc("GET /{$}", handleHome)

	// 语法
	mux.HandleFunc("GET /grammars", handleGrammarList)
	mux.HandleFunc("GET /grammars/new", handleGrammarNew)
	mux.HandleFunc("POST /grammars", handleGrammarCreate)
	mux.HandleFunc("POST /grammars/delete", handleGrammarBatchDelete)
	mux.HandleFunc("GET /grammars/review", handleGrammarReview)
	mux.HandleFunc("GET /grammars/{id}", handleGrammarDetail)
	mux.HandleFunc("GET /grammars/{id}/edit", handleGrammarEdit)
	mux.HandleFunc("POST /grammars/{id}", handleGrammarUpdate)
	mux.HandleFunc("POST /grammars/{id}/delete", handleGrammarDelete)
	mux.HandleFunc("GET /grammars/{id}/relate", handleGrammarRelatePage)
	mux.HandleFunc("POST /grammars/{id}/relations", handleGrammarRelateAdd)
	mux.HandleFunc("POST /grammars/{id}/relations/{rid}/delete", handleGrammarRelateDelete)

	// 单词
	mux.HandleFunc("GET /words", handleWordList)
	mux.HandleFunc("GET /words/new", handleWordNew)
	mux.HandleFunc("POST /words", handleWordCreate)
	mux.HandleFunc("POST /words/delete", handleWordBatchDelete)
	mux.HandleFunc("GET /words/{id}", handleWordDetail)
	mux.HandleFunc("GET /words/{id}/edit", handleWordEdit)
	mux.HandleFunc("POST /words/{id}", handleWordUpdate)
	mux.HandleFunc("POST /words/{id}/delete", handleWordDelete)
	mux.HandleFunc("GET /words/{id}/relate", handleWordRelatePage)
	mux.HandleFunc("POST /words/{id}/relations", handleWordRelateAdd)
	mux.HandleFunc("POST /words/{id}/relations/{rid}/delete", handleWordRelateDelete)

	// 单词测试
	mux.HandleFunc("GET /test", handleTestSetup)
	mux.HandleFunc("GET /test/run", handleTestRun)
	mux.HandleFunc("POST /test/finish", handleTestFinish)
	mux.HandleFunc("GET /test/result", handleTestResult)

	// 易错单词
	mux.HandleFunc("GET /error-words", handleErrorWordList)
	mux.HandleFunc("GET /error-words/add", handleErrorWordAddPage)
	mux.HandleFunc("POST /error-words/add", handleErrorWordAdd)
	mux.HandleFunc("POST /error-words/{id}/delete", handleErrorWordDelete)

	mux.Handle("GET /static/", http.FileServerFS(assets))

	// API
	mux.HandleFunc("GET /api/words/search", handleWordSearchAPI)
	mux.HandleFunc("GET /api/grammars/search", handleGrammarSearchAPI)

	log.Println("服务已启动: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
