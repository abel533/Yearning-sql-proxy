package proxy

import (
	"github.com/go-mysql-org/go-mysql/mysql"

	"github.com/toolsHelp/Yearning-sql-proxy/internal/config"
)

// rewriteSchemaResult 把结果里刚好等于真实库名的单元格换成显示名。
// 只做整格相等匹配，避免改动视图定义等更长的文本。没有映射或没有命中时原样返回。
func rewriteSchemaResult(n config.SchemaNames, res *mysql.Result) *mysql.Result {
	if n.Empty() || res == nil || res.Resultset == nil || len(res.RowDatas) == 0 {
		return res
	}
	fields := res.Fields
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f.Name)
	}
	rows := make([][]interface{}, len(res.RowDatas))
	changed := false
	dst := make([]mysql.FieldValue, len(fields))
	for r, rd := range res.RowDatas {
		vals, err := rd.ParseText(fields, dst)
		if err != nil {
			return res
		}
		row := make([]interface{}, len(vals))
		for i, fv := range vals {
			if fv.Type == mysql.FieldValueTypeString {
				s := string(fv.AsString())
				if disp := n.Display(s); disp != s {
					row[i] = disp
					changed = true
					continue
				}
				row[i] = s
				continue
			}
			row[i] = fv.Value()
		}
		rows[r] = row
	}
	if !changed {
		return res
	}
	rs, err := mysql.BuildSimpleTextResultset(names, rows)
	if err != nil {
		return res
	}
	setUTF8Charset(rs)
	return &mysql.Result{Status: res.Status, Warnings: res.Warnings, Resultset: rs}
}
