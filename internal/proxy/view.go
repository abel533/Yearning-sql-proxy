package proxy

import (
	"fmt"
	"log"
	"strings"

	"github.com/go-mysql-org/go-mysql/mysql"
)

// handleShowCreateView 拦截 SHOW CREATE VIEW。
//
// Yearning 使用的查询账号往往没有 SHOW VIEW。直接转发时 MySQL 返回
// 1142（SHOW VIEW command denied）。DataGrip 内省会把这个错误视为失败，
// 于是库里的表名在编辑器里变红，自动补全也不再出现。
// information_schema.views 不需要这条权限；定义为空时仍返回一行成功结果，
// 避免客户端收到错误。空定义不编造 SELECT 列，免得覆盖真实列信息。
func (h *Handler) handleShowCreateView(q, firstWord string) (*mysql.Result, bool, error) {
	if firstWord != "show" {
		return nil, false, nil
	}
	lower := strings.ToLower(q)
	const kw = "show create view"
	i := strings.Index(lower, kw)
	if i < 0 {
		return nil, false, nil
	}
	db, name := parseRoutineName(q[i+len(kw):])
	if name == "" {
		return nil, false, nil
	}
	if db == "" {
		h.mu.Lock()
		db = h.schema
		h.mu.Unlock()
	}
	cols := showCreateViewCols()
	if !isRealSchema(db) {
		return emptyResultWithCols(cols), true, nil
	}

	log.Printf("本地改写 SHOW CREATE VIEW: %s.%s", db, name)
	rq := viewLookupQuery(db, name)
	targets, err := h.targetSources(db, rq)
	if err != nil {
		log.Printf("SHOW CREATE VIEW 定位数据源失败，返回空定义: %v", err)
		return viewCreateResult(db, name, "", "", "", "", "", ""), true, nil
	}
	raw, _, err := h.execAcross(rq, db, targets)
	if err != nil || raw == nil || len(raw.Results) == 0 || len(raw.Results[0].Data) == 0 {
		if err != nil {
			log.Printf("SHOW CREATE VIEW 读取 information_schema.views 失败，返回空定义: %v", err)
		}
		return viewCreateResult(db, name, "", "", "", "", "", ""), true, nil
	}
	row := raw.Results[0].Data[0]
	return viewCreateResult(db, name,
		viewCell(row, "definer"),
		viewCell(row, "security_type"),
		viewCell(row, "view_definition"),
		viewCell(row, "check_option"),
		viewCell(row, "cs_client"),
		viewCell(row, "coll_conn"),
	), true, nil
}

func showCreateViewCols() []string {
	return []string{"View", "Create View", "character_set_client", "collation_connection"}
}

func viewLookupQuery(db, name string) string {
	esc := strings.NewReplacer("'", "''", "\\", "\\\\")
	return fmt.Sprintf(`SELECT DEFINER AS definer, SECURITY_TYPE AS security_type, `+
		`VIEW_DEFINITION AS view_definition, CHECK_OPTION AS check_option, `+
		`CHARACTER_SET_CLIENT AS cs_client, COLLATION_CONNECTION AS coll_conn `+
		`FROM information_schema.views WHERE TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'`,
		esc.Replace(db), esc.Replace(name))
}

func viewCell(row map[string]interface{}, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// buildViewDDL 拼 SHOW CREATE VIEW 的 Create View 列。
// definition 为空时不写 AS 子句，避免 DataGrip 用假列覆盖 information_schema.columns。
func buildViewDDL(db, name, definer, security, definition, checkOption string) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if i := strings.Index(definer, "@"); i >= 0 {
		fmt.Fprintf(&b, "DEFINER=`%s`@`%s` ", definer[:i], definer[i+1:])
	}
	if security != "" {
		b.WriteString("SQL SECURITY ")
		b.WriteString(strings.ToUpper(security))
		b.WriteByte(' ')
	}
	fmt.Fprintf(&b, "VIEW `%s`.`%s`", db, name)
	if strings.TrimSpace(definition) != "" {
		b.WriteString(" AS ")
		b.WriteString(definition)
		if checkOption != "" && !strings.EqualFold(checkOption, "NONE") {
			b.WriteString(" WITH ")
			b.WriteString(strings.ToUpper(checkOption))
			b.WriteString(" CHECK OPTION")
		}
	}
	return b.String()
}

func viewCreateResult(db, name, definer, security, definition, checkOption, csClient, collConn string) *mysql.Result {
	cols := showCreateViewCols()
	ddl := buildViewDDL(db, name, definer, security, definition, checkOption)
	rs, err := mysql.BuildSimpleTextResultset(cols, [][]interface{}{
		{name, ddl, csClient, collConn},
	})
	if err != nil {
		return emptyResultWithCols(cols)
	}
	setUTF8Charset(rs)
	return &mysql.Result{Status: 0x0002, Resultset: rs}
}
