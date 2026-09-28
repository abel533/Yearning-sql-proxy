package config

import "testing"

func testNames(t *testing.T) SchemaNames {
	t.Helper()
	n, err := NewSchemaNames(map[string]string{
		"tem658982298981219327": "企业A",
		"tem658982298981219333": "企业B",
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestToRealSchemaSQL(t *testing.T) {
	n := testNames(t)
	cases := []struct{ in, want string }{
		{"select * from 企业A.table_a", "select * from tem658982298981219327.table_a"},
		{"select * from `企业A`.`table_a`", "select * from `tem658982298981219327`.`table_a`"},
		{"select * from information_schema.tables where table_schema = '企业A'",
			"select * from information_schema.tables where table_schema = 'tem658982298981219327'"},
		{"USE `企业B`", "USE `tem658982298981219333`"},
		{"select * from tem658982298981219327.table_a", "select * from tem658982298981219327.table_a"},
		{"select * from 企业A_extra.t", "select * from 企业A_extra.t"},
		{"select '企业A是备注'", "select '企业A是备注'"},
	}
	for _, c := range cases {
		if got := n.ToReal(c.in); got != c.want {
			t.Errorf("ToReal(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSchemaNamesRejectsDuplicateDisplay(t *testing.T) {
	_, err := NewSchemaNames(map[string]string{
		"db1": "企业A",
		"db2": "企业A",
	})
	if err == nil {
		t.Fatal("重复显示名应拒绝")
	}
}
