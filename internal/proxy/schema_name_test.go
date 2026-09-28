package proxy

import (
	"testing"

	"github.com/go-mysql-org/go-mysql/mysql"

	"github.com/toolsHelp/Yearning-sql-proxy/internal/config"
)

func TestRewriteSchemaResult(t *testing.T) {
	n, err := config.NewSchemaNames(map[string]string{
		"tem658982298981219327": "企业A",
	})
	if err != nil {
		t.Fatal(err)
	}
	rs, err := mysql.BuildSimpleTextResultset(
		[]string{"table_schema", "table_name"},
		[][]interface{}{
			{"tem658982298981219327", "table_a"},
			{"other_db", "table_b"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	setUTF8Charset(rs)
	out := rewriteSchemaResult(n, &mysql.Result{Status: 0x0002, Resultset: rs})
	got := schemaResultCells(t, out)
	if got[0][0] != "企业A" || got[0][1] != "table_a" {
		t.Fatalf("映射行 = %v", got[0])
	}
	if got[1][0] != "other_db" {
		t.Fatalf("未配置的库被改了: %v", got[1])
	}
}

func schemaResultCells(t *testing.T, res *mysql.Result) [][]string {
	t.Helper()
	dst := make([]mysql.FieldValue, len(res.Fields))
	var rows [][]string
	for _, rd := range res.RowDatas {
		vals, err := rd.ParseText(res.Fields, dst)
		if err != nil {
			t.Fatal(err)
		}
		row := make([]string, len(vals))
		for i, fv := range vals {
			if fv.Type == mysql.FieldValueTypeString {
				row[i] = string(fv.AsString())
			}
		}
		rows = append(rows, row)
	}
	return rows
}
