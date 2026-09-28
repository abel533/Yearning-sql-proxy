package proxy

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/toolsHelp/Yearning-sql-proxy/internal/msgpack"
)

// yearningQueryLimit 是 Yearning 查询设置的默认行数上限。
// 引擎会给没有 LIMIT 的 SELECT 补上这个上限。审计里表清单、列、索引都正好 1000 行，
// 而对象总数是 1626、列关联行是 41397，DataGrip 因此缺了后半段表和列，编辑器里不提示。
const yearningQueryLimit = 1000

// maxMetaPages 防止 OFFSET 失效时无限翻页。200 页即 20 万行，够 information_schema 使用。
const maxMetaPages = 200

// expandMetaPages 在单数据源的 information_schema 查询被上限截断时继续翻页并合并。
// 只处理返回行数正好等于上限的结果；恰好满页时下一页为空，不会多出数据。
// 服务端若丢掉 OFFSET，下一页会与上一页重复，此时停在已有结果上。
func (h *Handler) expandMetaPages(q, schema string, targets []string, raw *msgpack.QueryResults) *msgpack.QueryResults {
	if len(targets) != 1 || raw == nil || len(raw.Results) != 1 || raw.Results[0] == nil {
		return raw
	}
	if !isPageableMeta(q) || hasOuterClause(q, "LIMIT") {
		return raw
	}
	base := raw.Results[0]
	if len(base.Data) != yearningQueryLimit {
		return raw
	}

	// 第一页如果没有 ORDER BY，引擎补的 LIMIT 与后面带 ORDER BY 的分页对不齐，整段重拉。
	if !hasOuterClause(q, "ORDER BY") {
		next, _, err := h.execAcross(pageSQL(q, yearningQueryLimit, 0), schema, targets)
		if err != nil || next == nil || len(next.Results) == 0 || next.Results[0] == nil {
			log.Printf("元数据按序重拉第一页失败，保留已有 %d 行: %v", len(base.Data), err)
			return raw
		}
		base.Field = next.Results[0].Field
		base.Data = next.Results[0].Data
		if len(base.Data) < yearningQueryLimit {
			return raw
		}
	}

	seen := map[string]struct{}{}
	if len(base.Data) > 0 {
		seen[rowSig(base.Data[0])] = struct{}{}
	}
	offset := len(base.Data)
	for page := 0; page < maxMetaPages; page++ {
		next, _, err := h.execAcross(pageSQL(q, yearningQueryLimit, offset), schema, targets)
		if err != nil {
			log.Printf("元数据分页失败 offset=%d，保留已有 %d 行: %v", offset, len(base.Data), err)
			break
		}
		if next == nil || len(next.Results) == 0 || next.Results[0] == nil || len(next.Results[0].Data) == 0 {
			break
		}
		rows := next.Results[0].Data
		sig := rowSig(rows[0])
		if _, dup := seen[sig]; dup {
			log.Printf("元数据分页 OFFSET 未生效，停止在 %d 行", len(base.Data))
			break
		}
		seen[sig] = struct{}{}
		base.Data = append(base.Data, rows...)
		offset += len(rows)
		if len(rows) < yearningQueryLimit {
			break
		}
	}
	log.Printf("元数据分页完成，共 %d 行", len(base.Data))
	return raw
}

func isPageableMeta(q string) bool {
	return strings.Contains(strings.ToLower(q), "information_schema.")
}

// pageSQL 在语句末尾加上稳定排序和分页。已有外层 ORDER BY 时不再追加。
func pageSQL(q string, limit, offset int) string {
	s := strings.TrimSpace(q)
	s = strings.TrimRight(s, "; \t\r\n")
	if !hasOuterClause(s, "ORDER BY") {
		s += " ORDER BY 1"
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", s, limit, offset)
}

// hasOuterClause 判断关键字是否出现在括号和引号之外。
func hasOuterClause(s, keyword string) bool {
	upper := strings.ToUpper(s)
	kw := strings.ToUpper(keyword)
	depth := 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && strings.HasPrefix(upper[i:], kw) {
				return true
			}
		}
	}
	return false
}

func rowSig(row map[string]interface{}) string {
	if len(row) == 0 {
		return ""
	}
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%v\x1f", k, row[k])
	}
	return b.String()
}
