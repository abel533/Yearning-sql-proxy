package proxy

import (
	"strings"
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"
)

func TestHandleShowCreateViewNotHandled(t *testing.T) {
	h := &Handler{}
	for _, q := range []string{
		"SHOW CREATE TABLE `db`.`t`",
		"SHOW CREATE PROCEDURE p",
		"SELECT 1",
	} {
		if _, handled, _ := h.handleShowCreateView(q, "show"); handled && !strings.HasPrefix(strings.ToLower(q), "show create view") {
			t.Errorf("handleShowCreateView(%q) 不应处理", q)
		}
	}
	if _, handled, _ := h.handleShowCreateView("SHOW CREATE VIEW v", "select"); handled {
		t.Fatal("非 show 语句不应处理")
	}
}

func TestHandleShowCreateViewUnknownSchema(t *testing.T) {
	h := &Handler{}
	r, handled, err := h.handleShowCreateView("SHOW CREATE VIEW only_name", "show")
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if r == nil || r.Resultset == nil || len(r.RowDatas) != 0 {
		t.Fatalf("未知库应返回空结果，rows=%d", rowsOf(r))
	}
}

func TestBuildViewDDL(t *testing.T) {
	got := buildViewDDL("db", "v_order", "app@%", "DEFINER", "select `id` from `t`", "CASCADED")
	want := "CREATE DEFINER=`app`@`%` SQL SECURITY DEFINER VIEW `db`.`v_order` AS select `id` from `t` WITH CASCADED CHECK OPTION"
	if got != want {
		t.Fatalf("got %s", got)
	}
	// 没有定义时不编造 AS，避免假列覆盖真实列信息。
	empty := buildViewDDL("db", "v_order", "", "", "", "")
	if strings.Contains(empty, " AS ") {
		t.Fatalf("空定义不应带 AS: %s", empty)
	}
	if empty != "CREATE VIEW `db`.`v_order`" {
		t.Fatalf("got %s", empty)
	}
}

func TestViewLookupQueryEscapes(t *testing.T) {
	q := viewLookupQuery("db'", "v\\")
	if !strings.Contains(q, "TABLE_SCHEMA = 'db'''") || !strings.Contains(q, "TABLE_NAME = 'v\\\\'") {
		t.Fatalf("未转义: %s", q)
	}
}

func TestRowsOfUsesRowDatas(t *testing.T) {
	rs, err := mysql.BuildSimpleTextResultset([]string{"a"}, [][]interface{}{{"1"}, {"2"}})
	if err != nil {
		t.Fatal(err)
	}
	res := &mysql.Result{Resultset: rs}
	if rowsOf(res) != 2 {
		t.Fatalf("rowsOf = %d, want 2", rowsOf(res))
	}
}
