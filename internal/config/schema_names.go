package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SchemaNames 是库名的双向映射。
// 真实库名用于转发 Yearning；显示名只出现在客户端看到的结果和客户端写来的 SQL 里。
type SchemaNames struct {
	realToDisp map[string]string
	dispToReal map[string]string
}

// NewSchemaNames 从配置构造映射。空配置返回零值，调用方当作未启用。
func NewSchemaNames(mapping map[string]string) (SchemaNames, error) {
	if len(mapping) == 0 {
		return SchemaNames{}, nil
	}
	n := SchemaNames{
		realToDisp: make(map[string]string, len(mapping)),
		dispToReal: make(map[string]string, len(mapping)),
	}
	for real, disp := range mapping {
		real = strings.TrimSpace(real)
		disp = strings.TrimSpace(disp)
		if real == "" || disp == "" {
			return SchemaNames{}, fmt.Errorf("mapping 的库名和显示名都不能为空")
		}
		if real == disp {
			continue
		}
		if other, ok := n.dispToReal[disp]; ok {
			return SchemaNames{}, fmt.Errorf("mapping 显示名 %q 同时对应 %q 和 %q", disp, other, real)
		}
		if _, ok := n.realToDisp[disp]; ok {
			return SchemaNames{}, fmt.Errorf("mapping 显示名 %q 与另一个真实库名相同", disp)
		}
		n.realToDisp[real] = disp
		n.dispToReal[disp] = real
	}
	return n, nil
}

// Empty 表示没有启用库名映射。
func (n SchemaNames) Empty() bool {
	return len(n.realToDisp) == 0
}

// Display 把真实库名换成显示名。没有映射时原样返回。
func (n SchemaNames) Display(real string) string {
	if disp, ok := n.realToDisp[real]; ok {
		return disp
	}
	return real
}

// Real 把显示名换成真实库名。没有映射时原样返回。
func (n SchemaNames) Real(name string) string {
	if real, ok := n.dispToReal[name]; ok {
		return real
	}
	return name
}

// ToReal 把 SQL 里的显示名换回真实库名。
// 只替换完整标识符和成对引号里的完整字符串，避免改到更长的名字或字面量片段。
func (n SchemaNames) ToReal(sql string) string {
	if n.Empty() || sql == "" {
		return sql
	}
	var b strings.Builder
	b.Grow(len(sql))
	for i := 0; i < len(sql); {
		c := sql[i]
		if c == '\'' || c == '"' || c == '`' {
			content, next, ok := readSQLQuoted(sql, i)
			if !ok {
				b.WriteString(sql[i:])
				break
			}
			if real, hit := n.dispToReal[content]; hit {
				b.WriteByte(c)
				b.WriteString(real)
				b.WriteByte(c)
			} else {
				b.WriteString(sql[i:next])
			}
			i = next
			continue
		}
		r, size := utf8.DecodeRuneInString(sql[i:])
		if isIdentStart(r) {
			j := i + size
			for j < len(sql) {
				r2, sz := utf8.DecodeRuneInString(sql[j:])
				if !isIdentPart(r2) {
					break
				}
				j += sz
			}
			word := sql[i:j]
			if real, hit := n.dispToReal[word]; hit {
				b.WriteString(real)
			} else {
				b.WriteString(word)
			}
			i = j
			continue
		}
		b.WriteString(sql[i : i+size])
		i += size
	}
	return b.String()
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// readSQLQuoted 读取从 i 开始的引号字符串。MySQL 用成对引号转义引号本身。
func readSQLQuoted(s string, i int) (content string, next int, ok bool) {
	q := s[i]
	var b strings.Builder
	for j := i + 1; j < len(s); j++ {
		if s[j] == q {
			if j+1 < len(s) && s[j+1] == q {
				b.WriteByte(q)
				j++
				continue
			}
			return b.String(), j + 1, true
		}
		if q != '`' && s[j] == '\\' && j+1 < len(s) {
			b.WriteByte(s[j+1])
			j++
			continue
		}
		b.WriteByte(s[j])
	}
	return "", i, false
}
