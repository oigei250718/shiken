// 一次性迁移工具：把 SQLite shiken.db 的全部数据导入 MySQL（保留所有 ID）。
// 用法: go run .
package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

const mysqlDSN = "root:123456@tcp(100.108.142.7:3306)/shiken?charset=utf8mb4&collation=utf8mb4_unicode_ci&multiStatements=true"

const schema = `
CREATE TABLE IF NOT EXISTS words (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  word       VARCHAR(255) NOT NULL,
  kana       VARCHAR(255) NOT NULL DEFAULT '',
  level      TINYINT NOT NULL DEFAULT 3,
  created_at VARCHAR(19) NOT NULL,
  updated_at VARCHAR(19) NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS word_meanings (
  id      INT UNSIGNED NOT NULL AUTO_INCREMENT,
  word_id INT UNSIGNED NOT NULL,
  meaning TEXT NOT NULL,
  sort    INT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_word_meanings_word (word_id),
  CONSTRAINT fk_word_meanings_word FOREIGN KEY (word_id) REFERENCES words(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS meaning_examples (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  meaning_id INT UNSIGNED NOT NULL,
  example    TEXT NOT NULL,
  sort       INT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_meaning_examples_meaning (meaning_id),
  CONSTRAINT fk_meaning_examples_meaning FOREIGN KEY (meaning_id) REFERENCES word_meanings(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS word_relations (
  a INT UNSIGNED NOT NULL,
  b INT UNSIGNED NOT NULL,
  PRIMARY KEY (a, b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS error_words (
  word_id    INT UNSIGNED NOT NULL,
  created_at VARCHAR(19) NOT NULL,
  PRIMARY KEY (word_id),
  CONSTRAINT fk_error_words_word FOREIGN KEY (word_id) REFERENCES words(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS grammars (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  format     VARCHAR(255) NOT NULL,
  level      TINYINT NOT NULL DEFAULT 3,
  created_at VARCHAR(19) NOT NULL,
  updated_at VARCHAR(19) NOT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS grammar_meanings (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  grammar_id INT UNSIGNED NOT NULL,
  meaning    TEXT NOT NULL,
  sort       INT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_grammar_meanings_grammar (grammar_id),
  CONSTRAINT fk_grammar_meanings_grammar FOREIGN KEY (grammar_id) REFERENCES grammars(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS grammar_meaning_examples (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  meaning_id INT UNSIGNED NOT NULL,
  example    TEXT NOT NULL,
  sort       INT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_grammar_meaning_examples_meaning (meaning_id),
  CONSTRAINT fk_grammar_meaning_examples_meaning FOREIGN KEY (meaning_id) REFERENCES grammar_meanings(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS grammar_relations (
  a INT UNSIGNED NOT NULL,
  b INT UNSIGNED NOT NULL,
  PRIMARY KEY (a, b)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`

// 表名 → 列（与两端 schema 一致；迁移顺序保证父表先于子表）
var tableCols = []struct {
	table string
	cols  string
}{
	{"words", "id, word, kana, level, created_at, updated_at"},
	{"word_meanings", "id, word_id, meaning, sort"},
	{"meaning_examples", "id, meaning_id, example, sort"},
	{"word_relations", "a, b"},
	{"error_words", "word_id, created_at"},
	{"grammars", "id, format, level, created_at, updated_at"},
	{"grammar_meanings", "id, grammar_id, meaning, sort"},
	{"grammar_meaning_examples", "id, meaning_id, example, sort"},
	{"grammar_relations", "a, b"},
}

// 需要 AUTO_INCREMENT 对齐的表 → 对应自增列所在表名
var autoIncTables = []string{"words", "word_meanings", "meaning_examples", "grammars", "grammar_meanings", "grammar_meaning_examples"}

func main() {
	sqlite, err := sql.Open("sqlite", "/Users/wangweiyi/work/shiken/shiken.db")
	if err != nil {
		log.Fatal("打开 SQLite 失败: ", err)
	}
	defer sqlite.Close()

	my, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		log.Fatal("打开 MySQL 失败: ", err)
	}
	defer my.Close()
	if err := my.Ping(); err != nil {
		log.Fatal("连接 MySQL 失败: ", err)
	}

	// 建表
	if _, err := my.Exec(schema); err != nil {
		log.Fatal("MySQL 建表失败: ", err)
	}

	// 检查 MySQL 是否已有数据（防重复导入）
	var existing int
	if err := my.QueryRow(`SELECT COUNT(*) FROM words`).Scan(&existing); err != nil {
		log.Fatal(err)
	}
	if existing > 0 {
		log.Fatalf("MySQL words 表已有 %d 行数据，为避免重复导入已中止。如需重导请先清空 shiken 库。", existing)
	}

	tx, err := my.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	for _, t := range tableCols {
		n := migrateTable(tx, sqlite, t.table, t.cols)
		fmt.Printf("%-26s %d 行\n", t.table, n)
	}

	// 对齐自增起始值
	for _, t := range autoIncTables {
		var maxID int64
		if err := tx.QueryRow("SELECT COALESCE(MAX(id),0) FROM " + t).Scan(&maxID); err != nil {
			log.Fatal(err)
		}
		if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = %d", t, maxID+1)); err != nil {
			log.Fatal("设置 AUTO_INCREMENT 失败: ", t, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Fatal("提交失败: ", err)
	}
	fmt.Println("迁移完成 ✓")
}

// migrateTable 把 SQLite 一张表的数据按列名原样复制到 MySQL（同事务），返回行数
func migrateTable(tx *sql.Tx, sqlite *sql.DB, table, cols string) int64 {
	rows, err := sqlite.Query("SELECT " + cols + " FROM " + table)
	if err != nil {
		log.Fatalf("读取 SQLite %s 失败: %v", table, err)
	}
	defer rows.Close()

	colCount := countCols(cols)
	marks := placeholders(colCount)
	stmt, err := tx.Prepare("INSERT INTO " + table + " (" + cols + ") VALUES (" + marks + ")")
	if err != nil {
		log.Fatalf("预编译 %s 失败: %v", table, err)
	}
	defer stmt.Close()

	vals := make([]any, colCount)
	ptrs := make([]any, colCount)
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	var n int64
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			log.Fatalf("扫描 %s 失败: %v", table, err)
		}
		// SQLite 动态类型统一转 string/int 再写入，避免 driver 类型不兼容
		args := make([]any, colCount)
		for i, v := range vals {
			switch x := v.(type) {
			case string:
				args[i] = x
			case int64:
				args[i] = x
			case float64:
				args[i] = int64(x)
			case []byte:
				args[i] = string(x)
			case nil:
				args[i] = nil
			default:
				args[i] = fmt.Sprint(x)
			}
		}
		if _, err := stmt.Exec(args...); err != nil {
			log.Fatalf("写入 %s 失败: %v", table, err)
		}
		n++
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("遍历 %s 失败: %v", table, err)
	}
	return n
}

func countCols(cols string) int {
	n := 1
	for i := 0; i < len(cols); i++ {
		if cols[i] == ',' {
			n++
		}
	}
	return n
}

func placeholders(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += "?"
	}
	return s
}
