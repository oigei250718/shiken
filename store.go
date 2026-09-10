package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
)

const pageSize = 20

// ===================== 语法 =====================

// listGrammars 按录入时间倒序分页 + 模糊搜索（格式/含义/例句）
func listGrammars(q string, page int) ([]Grammar, int, error) {
	where := ""
	args := []any{}
	if q != "" {
		like := "%" + q + "%"
		where = `WHERE g.format LIKE ?
		  OR EXISTS (SELECT 1 FROM grammar_meanings m WHERE m.grammar_id = g.id AND m.meaning LIKE ?)
		  OR EXISTS (SELECT 1 FROM grammar_meaning_examples e
		              JOIN grammar_meanings m ON m.id = e.meaning_id
		              WHERE m.grammar_id = g.id AND e.example LIKE ?)`
		args = append(args, like, like, like)
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM grammars g `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := db.Query(`SELECT g.id, g.format, g.level, g.created_at, g.updated_at
		FROM grammars g `+where+` ORDER BY g.created_at DESC, g.id DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Grammar
	for rows.Next() {
		var g Grammar
		if err := rows.Scan(&g.ID, &g.Format, &g.Level, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, 0, err
		}
		g.Meanings, _ = grammarMeanings(g.ID)
		out = append(out, g)
	}
	return out, total, rows.Err()
}

func grammarMeanings(id int64) ([]Meaning, error) {
	rows, err := db.Query(`SELECT id, meaning FROM grammar_meanings WHERE grammar_id=? ORDER BY sort, id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Meaning
	for rows.Next() {
		var m Meaning
		var mid int64
		if err := rows.Scan(&mid, &m.Text); err != nil {
			return nil, err
		}
		m.Examples, _ = grammarMeaningExamples(mid)
		out = append(out, m)
	}
	return out, rows.Err()
}

func grammarMeaningExamples(meaningID int64) ([]string, error) {
	rows, err := db.Query(`SELECT example FROM grammar_meaning_examples WHERE meaning_id=? ORDER BY sort, id`, meaningID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func getGrammar(id int64) (*Grammar, error) {
	var g Grammar
	err := db.QueryRow(`SELECT id, format, level, created_at, updated_at FROM grammars WHERE id=?`, id).
		Scan(&g.ID, &g.Format, &g.Level, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if g.Meanings, err = grammarMeanings(id); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT gr.id, gr.format FROM grammar_relations r
		JOIN grammars gr ON gr.id = CASE WHEN r.a=? THEN r.b ELSE r.a END
		WHERE r.a=? OR r.b=? ORDER BY gr.format`, id, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref GrammarRef
		if err := rows.Scan(&ref.ID, &ref.Format); err != nil {
			return nil, err
		}
		g.Related = append(g.Related, ref)
	}
	return &g, rows.Err()
}

func saveGrammarChildren(tx *sql.Tx, id int64, meanings []Meaning) error {
	if _, err := tx.Exec(`DELETE FROM grammar_meanings WHERE grammar_id=?`, id); err != nil {
		return err
	}
	for i, m := range meanings {
		if strings.TrimSpace(m.Text) == "" && len(m.Examples) == 0 {
			continue
		}
		res, err := tx.Exec(`INSERT INTO grammar_meanings (grammar_id, meaning, sort) VALUES (?,?,?)`, id, strings.TrimSpace(m.Text), i)
		if err != nil {
			return err
		}
		mid, _ := res.LastInsertId()
		for j, e := range m.Examples {
			if strings.TrimSpace(e) == "" {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO grammar_meaning_examples (meaning_id, example, sort) VALUES (?,?,?)`, mid, strings.TrimSpace(e), j); err != nil {
				return err
			}
		}
	}
	return nil
}

func createGrammar(g *Grammar) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	t := now()
	res, err := tx.Exec(`INSERT INTO grammars (format, level, created_at, updated_at) VALUES (?,?,?,?)`,
		g.Format, g.Level, t, t)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if err := saveGrammarChildren(tx, id, g.Meanings); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func updateGrammar(g *Grammar) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE grammars SET format=?, level=?, updated_at=? WHERE id=?`,
		g.Format, g.Level, now(), g.ID); err != nil {
		return err
	}
	if err := saveGrammarChildren(tx, g.ID, g.Meanings); err != nil {
		return err
	}
	return tx.Commit()
}

// deleteGrammars 删除语法及其关联关系（子表由外键级联删除）
func deleteGrammars(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM grammar_relations WHERE a IN (`+marks+`) OR b IN (`+marks+`)`, append(args, args...)...); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM grammars WHERE id IN (`+marks+`)`, args...); err != nil {
		return err
	}
	return tx.Commit()
}

func addGrammarRelation(id, rid int64) error {
	if id == rid {
		return fmt.Errorf("不能关联自身")
	}
	a, b := min(id, rid), max(id, rid)
	_, err := db.Exec(`INSERT IGNORE INTO grammar_relations (a, b) VALUES (?,?)`, a, b)
	return err
}

func deleteGrammarRelation(id, rid int64) error {
	a, b := min(id, rid), max(id, rid)
	_, err := db.Exec(`DELETE FROM grammar_relations WHERE a=? AND b=?`, a, b)
	return err
}

// grammarPrevNext 按录入时间倒序的上一条（更新）/下一条（更早）
func grammarPrevNext(id int64) (prev, next int64) {
	var created string
	if err := db.QueryRow(`SELECT created_at FROM grammars WHERE id=?`, id).Scan(&created); err != nil {
		return 0, 0
	}
	_ = db.QueryRow(`SELECT id FROM grammars WHERE (created_at > ? OR (created_at = ? AND id > ?))
		ORDER BY created_at ASC, id ASC LIMIT 1`, created, created, id).Scan(&prev)
	_ = db.QueryRow(`SELECT id FROM grammars WHERE (created_at < ? OR (created_at = ? AND id < ?))
		ORDER BY created_at DESC, id DESC LIMIT 1`, created, created, id).Scan(&next)
	return
}

// sampleGrammarsForReview 按库中级别分布等比例随机抽取 n 条
func sampleGrammarsForReview(n int) ([]Grammar, error) {
	rows, err := db.Query(`SELECT level, COUNT(*) FROM grammars GROUP BY level`)
	if err != nil {
		return nil, err
	}
	counts := map[int]int{}
	total := 0
	for rows.Next() {
		var lv, c int
		if err := rows.Scan(&lv, &c); err != nil {
			rows.Close()
			return nil, err
		}
		counts[lv] = c
		total += c
	}
	rows.Close()
	if total == 0 {
		return nil, nil
	}
	if total <= n {
		return randomGrammars(`SELECT id, format, level, created_at, updated_at FROM grammars ORDER BY RAND()`, total)
	}
	// 最大余数法按比例分配
	type lvQuota struct {
		level int
		quota int
		rem   float64
	}
	quotas := []lvQuota{}
	assigned := 0
	for lv, c := range counts {
		exact := float64(c) * float64(n) / float64(total)
		q := int(exact)
		quotas = append(quotas, lvQuota{lv, q, exact - float64(q)})
		assigned += q
	}
	// 补足剩余名额给余数最大的级别
	for assigned < n {
		best := -1
		for i := range quotas {
			if best == -1 || quotas[i].rem > quotas[best].rem ||
				(quotas[i].rem == quotas[best].rem && counts[quotas[i].level] > counts[quotas[best].level]) {
				best = i
			}
		}
		quotas[best].quota++
		quotas[best].rem = 0
		assigned++
	}
	var out []Grammar
	for _, q := range quotas {
		if q.quota <= 0 {
			continue
		}
		gs, err := randomGrammars(fmt.Sprintf(
			`SELECT id, format, level, created_at, updated_at FROM grammars WHERE level=%d ORDER BY RAND() LIMIT %d`,
			q.level, q.quota), q.quota)
		if err != nil {
			return nil, err
		}
		out = append(out, gs...)
	}
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out, nil
}

func randomGrammars(query string, limit int) ([]Grammar, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Grammar
	for rows.Next() {
		var g Grammar
		if err := rows.Scan(&g.ID, &g.Format, &g.Level, &g.CreatedAt, &g.UpdatedAt); err != nil {
			return nil, err
		}
		g.Meanings, _ = grammarMeanings(g.ID)
		out = append(out, g)
	}
	return out, rows.Err()
}

// ===================== 单词 =====================

// listWords 按录入时间倒序分页 + 模糊搜索（单词/假名/含义）；isErr 按当前用户判断
func listWords(q string, page, level int, userID int64) ([]Word, int, error) {
	conds := []string{}
	args := []any{}
	if q != "" {
		like := "%" + q + "%"
		conds = append(conds, `(w.word LIKE ? OR w.kana LIKE ?
		  OR EXISTS (SELECT 1 FROM word_meanings m WHERE m.word_id = w.id AND m.meaning LIKE ?))`)
		args = append(args, like, like, like)
	}
	if level >= 1 && level <= 5 {
		conds = append(conds, "w.level = ?")
		args = append(args, level)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM words w `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.Query(`SELECT w.id, w.word, w.kana, w.level, w.created_at, w.updated_at,
		EXISTS(SELECT 1 FROM error_words ew WHERE ew.word_id = w.id AND ew.user_id = ?)
		FROM words w `+where+` ORDER BY w.created_at DESC, w.id DESC LIMIT ? OFFSET ?`,
		append(append([]any{userID}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		var isErr int
		if err := rows.Scan(&w.ID, &w.Word, &w.Kana, &w.Level, &w.CreatedAt, &w.UpdatedAt, &isErr); err != nil {
			return nil, 0, err
		}
		w.IsError = isErr == 1
		w.Meanings, _ = wordMeanings(w.ID)
		out = append(out, w)
	}
	return out, total, rows.Err()
}

func wordMeanings(id int64) ([]Meaning, error) {
	rows, err := db.Query(`SELECT id, meaning FROM word_meanings WHERE word_id=? ORDER BY sort, id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Meaning
	for rows.Next() {
		var m Meaning
		var mid int64
		if err := rows.Scan(&mid, &m.Text); err != nil {
			return nil, err
		}
		m.Examples, _ = meaningExamples(mid)
		out = append(out, m)
	}
	return out, rows.Err()
}

func meaningExamples(meaningID int64) ([]string, error) {
	rows, err := db.Query(`SELECT example FROM meaning_examples WHERE meaning_id=? ORDER BY sort, id`, meaningID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func getWord(id, userID int64) (*Word, error) {
	var w Word
	var isErr int
	err := db.QueryRow(`SELECT w.id, w.word, w.kana, w.level, w.created_at, w.updated_at,
		EXISTS(SELECT 1 FROM error_words ew WHERE ew.word_id = w.id AND ew.user_id = ?)
		FROM words w WHERE w.id=?`, userID, id).
		Scan(&w.ID, &w.Word, &w.Kana, &w.Level, &w.CreatedAt, &w.UpdatedAt, &isErr)
	if err != nil {
		return nil, err
	}
	w.IsError = isErr == 1
	if w.Meanings, err = wordMeanings(id); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT wr.id, wr.word, wr.kana FROM word_relations r
		JOIN words wr ON wr.id = CASE WHEN r.a=? THEN r.b ELSE r.a END
		WHERE r.a=? OR r.b=? ORDER BY wr.word`, id, id, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref WordRef
		if err := rows.Scan(&ref.ID, &ref.Word, &ref.Kana); err != nil {
			return nil, err
		}
		w.Related = append(w.Related, ref)
	}
	return &w, rows.Err()
}

func saveWordChildren(tx *sql.Tx, id int64, meanings []Meaning) error {
	if _, err := tx.Exec(`DELETE FROM word_meanings WHERE word_id=?`, id); err != nil {
		return err
	}
	for i, m := range meanings {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		res, err := tx.Exec(`INSERT INTO word_meanings (word_id, meaning, sort) VALUES (?,?,?)`, id, strings.TrimSpace(m.Text), i)
		if err != nil {
			return err
		}
		mid, _ := res.LastInsertId()
		for j, e := range m.Examples {
			if strings.TrimSpace(e) == "" {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO meaning_examples (meaning_id, example, sort) VALUES (?,?,?)`, mid, strings.TrimSpace(e), j); err != nil {
				return err
			}
		}
	}
	return nil
}

func createWord(w *Word) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	t := now()
	res, err := tx.Exec(`INSERT INTO words (word, kana, level, created_at, updated_at) VALUES (?,?,?,?,?)`,
		w.Word, w.Kana, w.Level, t, t)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	if err := saveWordChildren(tx, id, w.Meanings); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func updateWord(w *Word) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE words SET word=?, kana=?, level=?, updated_at=? WHERE id=?`,
		w.Word, w.Kana, w.Level, now(), w.ID); err != nil {
		return err
	}
	if err := saveWordChildren(tx, w.ID, w.Meanings); err != nil {
		return err
	}
	return tx.Commit()
}

func deleteWords(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM word_relations WHERE a IN (`+marks+`) OR b IN (`+marks+`)`, append(args, args...)...); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM words WHERE id IN (`+marks+`)`, args...); err != nil {
		return err
	}
	return tx.Commit()
}

func addWordRelation(id, rid int64) error {
	if id == rid {
		return fmt.Errorf("不能关联自身")
	}
	a, b := min(id, rid), max(id, rid)
	_, err := db.Exec(`INSERT IGNORE INTO word_relations (a, b) VALUES (?,?)`, a, b)
	return err
}

func deleteWordRelation(id, rid int64) error {
	a, b := min(id, rid), max(id, rid)
	_, err := db.Exec(`DELETE FROM word_relations WHERE a=? AND b=?`, a, b)
	return err
}

func wordPrevNext(id int64) (prev, next int64) {
	var created string
	if err := db.QueryRow(`SELECT created_at FROM words WHERE id=?`, id).Scan(&created); err != nil {
		return 0, 0
	}
	_ = db.QueryRow(`SELECT id FROM words WHERE (created_at > ? OR (created_at = ? AND id > ?))
		ORDER BY created_at ASC, id ASC LIMIT 1`, created, created, id).Scan(&prev)
	_ = db.QueryRow(`SELECT id FROM words WHERE (created_at < ? OR (created_at = ? AND id < ?))
		ORDER BY created_at DESC, id DESC LIMIT 1`, created, created, id).Scan(&next)
	return
}

// randomWordsForTest 从指定级别中随机抽 n 个单词
func randomWordsForTest(levels []int, n int) ([]Word, error) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(levels)), ",")
	args := make([]any, 0, len(levels)+1)
	for _, l := range levels {
		args = append(args, l)
	}
	args = append(args, n)
	rows, err := db.Query(`SELECT id, word, kana, level, created_at, updated_at, 0 FROM words
		WHERE level IN (`+marks+`) ORDER BY RAND() LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		var isErr int
		if err := rows.Scan(&w.ID, &w.Word, &w.Kana, &w.Level, &w.CreatedAt, &w.UpdatedAt, &isErr); err != nil {
			return nil, err
		}
		w.Meanings, _ = wordMeanings(w.ID)
		out = append(out, w)
	}
	return out, rows.Err()
}

func getWordsByIDs(ids []int64) ([]Word, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.Query(`SELECT id, word, kana, level, created_at, updated_at, 1 FROM words WHERE id IN (`+marks+`) ORDER BY word`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		var isErr int
		if err := rows.Scan(&w.ID, &w.Word, &w.Kana, &w.Level, &w.CreatedAt, &w.UpdatedAt, &isErr); err != nil {
			return nil, err
		}
		w.Meanings, _ = wordMeanings(w.ID)
		out = append(out, w)
	}
	return out, rows.Err()
}

// ===================== 易错单词 =====================

func listErrorWords(page int, userID int64) ([]Word, int, error) {
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM error_words WHERE user_id=?`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.Query(`SELECT w.id, w.word, w.kana, w.level, w.created_at, w.updated_at
		FROM error_words ew JOIN words w ON w.id = ew.word_id
		WHERE ew.user_id = ?
		ORDER BY ew.created_at DESC, w.id DESC LIMIT ? OFFSET ?`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Word, &w.Kana, &w.Level, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, 0, err
		}
		w.IsError = true
		w.Meanings, _ = wordMeanings(w.ID)
		out = append(out, w)
	}
	return out, total, rows.Err()
}

func addErrorWord(userID, id int64) error {
	_, err := db.Exec(`INSERT IGNORE INTO error_words (user_id, word_id, created_at) VALUES (?,?,?)`, userID, id, now())
	return err
}

// deleteErrorWord 仅删除属于该用户的易错词（不能误删他人/共享数据）
func deleteErrorWord(userID, id int64) error {
	_, err := db.Exec(`DELETE FROM error_words WHERE user_id=? AND word_id=?`, userID, id)
	return err
}

// searchWordsNotInError 模糊搜索尚未加入「当前用户」易错库的单词（用于手动添加）
func searchWordsNotInError(q string, userID int64) ([]Word, error) {
	like := "%" + q + "%"
	rows, err := db.Query(`SELECT w.id, w.word, w.kana, w.level, w.created_at, w.updated_at FROM words w
		WHERE NOT EXISTS (SELECT 1 FROM error_words ew WHERE ew.word_id = w.id AND ew.user_id = ?)
		  AND (w.word LIKE ? OR w.kana LIKE ?
		    OR EXISTS (SELECT 1 FROM word_meanings m WHERE m.word_id = w.id AND m.meaning LIKE ?))
		ORDER BY w.created_at DESC, w.id DESC LIMIT 50`, userID, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Word, &w.Kana, &w.Level, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.Meanings, _ = wordMeanings(w.ID)
		out = append(out, w)
	}
	return out, rows.Err()
}

// ==================== 文章 ====================

// listArticles 文章列表（按创建时间降序），q 匹配标题或内容
func listArticles(q string, page int) ([]Article, int, error) {
	where := ""
	args := []any{}
	if q != "" {
		like := "%" + q + "%"
		where = "WHERE title LIKE ? OR content LIKE ?"
		args = append(args, like, like)
	}
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM articles `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := db.Query(`SELECT id, title, content, created_at, updated_at FROM articles `+where+
		` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.Title, &a.Content, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func getArticle(id int64) (Article, error) {
	var a Article
	err := db.QueryRow(`SELECT id, title, content, created_at, updated_at FROM articles WHERE id=?`, id).
		Scan(&a.ID, &a.Title, &a.Content, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func createArticle(a *Article) (int64, error) {
	now := now()
	res, err := db.Exec(`INSERT INTO articles (title, content, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		a.Title, a.Content, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func updateArticle(a *Article) error {
	_, err := db.Exec(`UPDATE articles SET title=?, content=?, updated_at=? WHERE id=?`,
		a.Title, a.Content, now(), a.ID)
	return err
}

func deleteArticle(id int64) error {
	_, err := db.Exec(`DELETE FROM articles WHERE id=?`, id)
	return err
}

// ==================== 用户 ====================

// createUser 创建用户，密码以 bcrypt 哈希存储；默认启用
func createUser(username, password, role string) (int64, error) {
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	res, err := db.Exec(`INSERT INTO users (username, password, role, disabled, created_at, updated_at)
		VALUES (?,?,?,0,?,?)`, username, hash, role, now(), now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// getUser 按 ID 加载用户（用于鉴权时取最新状态）
func getUser(id int64) (User, error) {
	var u User
	var disabled int
	err := db.QueryRow(`SELECT id, username, password, role, disabled, created_at, updated_at
		FROM users WHERE id=?`, id).
		Scan(&u.ID, &u.Username, &u.Password, &u.Role, &disabled, &u.CreatedAt, &u.UpdatedAt)
	u.Disabled = disabled != 0
	return u, err
}

// getUserByUsername 按用户名加载用户（用于登录校验）
func getUserByUsername(username string) (User, error) {
	var u User
	var disabled int
	err := db.QueryRow(`SELECT id, username, password, role, disabled, created_at, updated_at
		FROM users WHERE username=?`, username).
		Scan(&u.ID, &u.Username, &u.Password, &u.Role, &disabled, &u.CreatedAt, &u.UpdatedAt)
	u.Disabled = disabled != 0
	return u, err
}

// listUsers 列出全部用户（用户管理页）
func listUsers() ([]User, error) {
	rows, err := db.Query(`SELECT id, username, role, disabled, created_at, updated_at
		FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var disabled int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &disabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Disabled = disabled != 0
		out = append(out, u)
	}
	return out, rows.Err()
}

// updateUserRole 修改用户角色（root / admin / user）
func updateUserRole(id int64, role string) error {
	_, err := db.Exec(`UPDATE users SET role=?, updated_at=? WHERE id=?`, role, now(), id)
	return err
}

// setUserDisabled 启用 / 禁用用户
func setUserDisabled(id int64, disabled bool) error {
	v := 0
	if disabled {
		v = 1
	}
	_, err := db.Exec(`UPDATE users SET disabled=?, updated_at=? WHERE id=?`, v, now(), id)
	return err
}

// resetUserPassword 将用户密码重置为指定明文（bcrypt 哈希后存储）
func resetUserPassword(id int64, plain string) error {
	hash, err := hashPassword(plain)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE users SET password=?, updated_at=? WHERE id=?`, hash, now(), id)
	return err
}

// deleteUser 删除用户（其易错词由外键级联删除）
func deleteUser(id int64) error {
	_, err := db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}
