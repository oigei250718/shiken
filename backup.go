package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxBackups = 20

// backupDB 启动时用 mysqldump 把 MySQL 数据库导出到 backups/ 目录，保留最近 20 份。
// mysqldump 不可用时跳过（不影响启动）。
func backupDB() {
	dsn := os.Getenv("SHIKEN_MYSQL_DSN")
	if dsn == "" {
		dsn = mysqlDSN
	}
	host, port, user, pass, dbname := parseDSN(dsn)

	mysqldump, err := exec.LookPath("mysqldump")
	if err != nil {
		log.Printf("未找到 mysqldump，跳过启动备份")
		return
	}
	dir := "backups"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("创建备份目录失败: %v", err)
		return
	}
	dst := filepath.Join(dir, "shiken-"+time.Now().Format("20060102-150405")+".sql")
	f, err := os.Create(dst)
	if err != nil {
		log.Printf("创建备份文件失败: %v", err)
		return
	}
	defer f.Close()

	args := []string{"-h", host}
	if port != "" {
		args = append(args, "-P", port)
	}
	args = append(args, "-u", user, "-p"+pass,
		"--default-character-set=utf8mb4", "--single-transaction", dbname)
	cmd := exec.Command(mysqldump, args...)
	cmd.Stdout = f
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Printf("mysqldump 备份失败: %v", err)
		_ = os.Remove(dst)
		return
	}
	log.Printf("数据库已备份: %s", dst)

	// 清理旧备份，仅保留最近 maxBackups 份
	entries, err := filepath.Glob(filepath.Join(dir, "shiken-*.sql"))
	if err != nil || len(entries) <= maxBackups {
		return
	}
	sort.Strings(entries)
	for _, old := range entries[:len(entries)-maxBackups] {
		_ = os.Remove(old)
	}
}

// parseDSN 解析 go-sql-driver 格式 DSN: user:pass@tcp(host:port)/dbname?...
func parseDSN(dsn string) (host, port, user, pass, dbname string) {
	rest := dsn
	if i := strings.LastIndex(rest, "@tcp("); i >= 0 {
		cred := rest[:i]
		if j := strings.Index(cred, ":"); j >= 0 {
			user, pass = cred[:j], cred[j+1:]
		}
		rest = rest[i+5:]
		if j := strings.Index(rest, ")"); j >= 0 {
			hostport := rest[:j]
			if k := strings.LastIndex(hostport, ":"); k >= 0 {
				host, port = hostport[:k], hostport[k+1:]
			} else {
				host = hostport
			}
			rest = rest[j+1:]
		}
	}
	if i := strings.Index(rest, "/"); i >= 0 {
		dbname = rest[i+1:]
		if j := strings.Index(dbname, "?"); j >= 0 {
			dbname = dbname[:j]
		}
	}
	return
}
