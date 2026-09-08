package main

import "html/template"

// Meaning 单词的一个含义及其例句
type Meaning struct {
	Text     string
	Examples []string
}

// WordRef 关联单词的简要信息
type WordRef struct {
	ID   int64
	Word string
	Kana string
}

// Word 单词完整信息
type Word struct {
	ID        int64
	Word      string
	Kana      string
	Level     int
	Meanings  []Meaning
	Related   []WordRef
	IsError   bool
	CreatedAt string
	UpdatedAt string
}

// MeaningTexts 返回所有含义文本
func (w *Word) MeaningTexts() []string {
	out := make([]string, 0, len(w.Meanings))
	for _, m := range w.Meanings {
		out = append(out, m.Text)
	}
	return out
}

// GrammarRef 关联语法的简要信息
type GrammarRef struct {
	ID     int64
	Format string
}

// Grammar 语法完整信息
type Grammar struct {
	ID        int64
	Format    string
	Level     int
	Meanings  []Meaning
	Related   []GrammarRef
	CreatedAt string
	UpdatedAt string
}

// Page 分页信息
type Page struct {
	Current   int
	Total     int // 总页数
	Count     int // 总条数
	Query     string
	Extra     template.URL // 附加查询参数（内部生成，可信），如 "&level=3"
	HasPrev   bool
	HasNext   bool
}
