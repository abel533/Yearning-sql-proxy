// Package codec 负责与 Yearning 查询 WebSocket 之间 msgpack 的编解码。
//
// 对齐 Yearning 3.1.2（src/handler/personal/query.go）：
// 服务端把消息直接解进 QueryDeal.Ref，键为 type/sql/schema/source_id。
// type 取值与服务端 iota 一致：1 关闭、2 打开后端连接、3 心跳；
// 其余值（含历史的 4）走 SQL 执行分支。
// 响应为 queryResults。3.1.2 的 heartbeat 是 uint8（1 = ping 回包），
// 较新分支可能是字符串，两种都认。
package msgpack

import (
	"github.com/vmihailenco/msgpack/v5"
)

// 请求类型常量，与 Yearning 3.1.2 query.go 的 iota 一致。
const (
	TypeClose = 1
	TypeOpen  = 2 // QUERY_OPEN：让服务端连上数据源 MySQL，查询前必须先发
	TypePing  = 3 // QUERY_PING：msgpack 心跳。文本 "ping" 会被服务端当成坏包并断开
	// TypeQuery 落在服务端 switch 的 default 分支，真正执行 SQL。
	// 3.1.2 对 0 和 4 一视同仁；保留 4 以兼容原先按 type=4 识别查询的分支。
	TypeQuery = 4
)

// QueryDeal 是发往 Yearning 的查询请求。
// 服务端解进 Ref，msgpack 键名即字段名。
type QueryDeal struct {
	Type     int    `msgpack:"type"`
	SQL      string `msgpack:"sql"`
	Schema   string `msgpack:"schema"`
	SourceId string `msgpack:"source_id"`
}

// QueryResults 是 Yearning 返回的查询结果。
// 字段名与 Yearning queryResults 结构体的 msgpack tag 一致。
type QueryResults struct {
	Export    bool     `msgpack:"export"`
	Error     string   `msgpack:"error"`
	Results   []*Query `msgpack:"results"`
	QueryTime int      `msgpack:"query_time"`
	Status    bool     `msgpack:"status"`
	Heartbeat beat     `msgpack:"heartbeat"`
	IsOnly    bool     `msgpack:"is_only"`
}

// IsHeartbeat 判断这条回包是不是心跳，而不是查询结果。
func (r *QueryResults) IsHeartbeat() bool {
	return r != nil && r.Heartbeat.ping
}

// beat 同时接受 uint8（3.1.2，非 0 即心跳）和字符串（较新分支，非空即心跳）。
// 类型对不上时不能让整包解码失败，否则正常结果会被当成坏包一直等到连接断开。
type beat struct {
	ping bool
}

func (b *beat) UnmarshalMsgpack(data []byte) error {
	if len(data) == 0 || b == nil {
		return nil
	}
	var n uint64
	if err := msgpack.Unmarshal(data, &n); err == nil {
		b.ping = n != 0
		return nil
	}
	var s string
	if err := msgpack.Unmarshal(data, &s); err == nil {
		b.ping = s != ""
		return nil
	}
	return nil
}

// Query 是单个结果集：Field 为字段元信息，Data 为数据行。
type Query struct {
	Field []map[string]interface{} `msgpack:"field"`
	Data  []map[string]interface{} `msgpack:"data"`
}

// MarshalQuery 编码一条查询消息为 msgpack 二进制。
func MarshalQuery(sql, schema, sourceID string) ([]byte, error) {
	return msgpack.Marshal(&QueryDeal{
		Type: TypeQuery, SQL: sql, Schema: schema, SourceId: sourceID,
	})
}

// MarshalControl 编码打开连接 / 心跳 / 关闭这类无 SQL 的控制消息。
func MarshalControl(typ int) ([]byte, error) {
	return msgpack.Marshal(&QueryDeal{Type: typ})
}

// UnmarshalResults 解码 Yearning 返回的结果集。
func UnmarshalResults(b []byte) (*QueryResults, error) {
	r := new(QueryResults)
	if err := msgpack.Unmarshal(b, r); err != nil {
		return nil, err
	}
	return r, nil
}
