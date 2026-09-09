package main

import (
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

// mysqlDSN 默认连接串，可用环境变量 SHIKEN_MYSQL_DSN 覆盖
const mysqlDSN = "root:123456@tcp(100.108.142.7:3306)/shiken?charset=utf8mb4&collation=utf8mb4_unicode_ci&multiStatements=true"

func openDB(dsn string) error {
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	// 连接池与超时
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	if _, err = db.Exec(schema); err != nil {
		return err
	}
	return db.Ping()
}

func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

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

CREATE TABLE IF NOT EXISTS articles (
  id         INT UNSIGNED NOT NULL AUTO_INCREMENT,
  title      VARCHAR(255) NOT NULL,
  content    MEDIUMTEXT NOT NULL,
  created_at VARCHAR(19) NOT NULL,
  updated_at VARCHAR(19) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_articles_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`
