package proxy

import "testing"

func TestPageSQL(t *testing.T) {
	got := pageSQL("SELECT table_name FROM information_schema.tables WHERE table_schema = 'db'", 1000, 1000)
	want := "SELECT table_name FROM information_schema.tables WHERE table_schema = 'db' ORDER BY 1 LIMIT 1000 OFFSET 1000"
	if got != want {
		t.Fatalf("got %s", got)
	}
	ordered := "SELECT column_name FROM information_schema.columns WHERE table_schema = 'db' ORDER BY table_name, ordinal_position"
	got = pageSQL(ordered, 1000, 2000)
	if got != ordered+" LIMIT 1000 OFFSET 2000" {
		t.Fatalf("已有 ORDER BY 被改写: %s", got)
	}
}

func TestHasOuterClauseIgnoresSubquery(t *testing.T) {
	q := "SELECT * FROM (SELECT a FROM t ORDER BY a LIMIT 1) x"
	if hasOuterClause(q, "ORDER BY") || hasOuterClause(q, "LIMIT") {
		t.Fatal("子查询里的 ORDER BY/LIMIT 不应算外层")
	}
	if !hasOuterClause(q+" ORDER BY 1", "ORDER BY") {
		t.Fatal("外层 ORDER BY 应命中")
	}
}

func TestIsPageableMeta(t *testing.T) {
	if !isPageableMeta("select * from information_schema.columns where table_schema = 'db'") {
		t.Fatal("columns 应分页")
	}
	if isPageableMeta("select * from fk_applybill limit 10") {
		t.Fatal("业务查询不应分页")
	}
}
