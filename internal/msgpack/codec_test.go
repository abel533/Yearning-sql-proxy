package msgpack

import (
	"testing"

	"github.com/vmihailenco/msgpack/v5"
)

func TestMarshalQuery_fields(t *testing.T) {
	b, err := MarshalQuery("select * from t", "mydb", "src-1")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := msgpack.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	// msgpack 对小整数可能解码为 int8/int16/int32/int64，做数值比较。
	switch typ := m["type"].(type) {
	case int8:
		if int(typ) != TypeQuery {
			t.Errorf("type = %v (%T), 期望 %d", m["type"], m["type"], TypeQuery)
		}
	case int16:
		if int(typ) != TypeQuery {
			t.Errorf("type = %v (%T), 期望 %d", m["type"], m["type"], TypeQuery)
		}
	case int32:
		if int(typ) != TypeQuery {
			t.Errorf("type = %v (%T), 期望 %d", m["type"], m["type"], TypeQuery)
		}
	case int64:
		if int(typ) != TypeQuery {
			t.Errorf("type = %v (%T), 期望 %d", m["type"], m["type"], TypeQuery)
		}
	case uint64:
		if int(typ) != TypeQuery {
			t.Errorf("type = %v (%T), 期望 %d", m["type"], m["type"], TypeQuery)
		}
	default:
		t.Errorf("type = %v (%T), 期望 %d", m["type"], m["type"], TypeQuery)
	}
	if m["sql"] != "select * from t" {
		t.Errorf("sql = %v", m["sql"])
	}
	if m["schema"] != "mydb" {
		t.Errorf("schema = %v", m["schema"])
	}
	if m["source_id"] != "src-1" {
		t.Errorf("source_id = %v", m["source_id"])
	}
}

func TestMarshalControl_openAndPing(t *testing.T) {
	for _, typ := range []int{TypeOpen, TypePing, TypeClose} {
		b, err := MarshalControl(typ)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]interface{}
		if err := msgpack.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		got, ok := asInt(m["type"])
		if !ok || got != typ {
			t.Errorf("type = %v, 期望 %d", m["type"], typ)
		}
	}
}

func TestUnmarshalResults_heartbeatUint8(t *testing.T) {
	// 3.1.2 queryResults.HeartBeat 是 uint8：0 是正常结果，1 是 ping 回包。
	raw := map[string]interface{}{
		"export":     false,
		"error":      "",
		"query_time": 3,
		"status":     false,
		"heartbeat":  uint8(0),
		"is_only":    false,
		"results": []interface{}{
			map[string]interface{}{
				"field": []interface{}{map[string]interface{}{"title": "id", "dataIndex": "id"}},
				"data":  []interface{}{map[string]interface{}{"id": "1"}},
			},
		},
	}
	b, err := msgpack.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := UnmarshalResults(b)
	if err != nil {
		t.Fatalf("uint8 heartbeat 解码失败: %v", err)
	}
	if res.IsHeartbeat() {
		t.Fatal("heartbeat=0 不应视为心跳")
	}
	if len(res.Results) != 1 || len(res.Results[0].Data) != 1 {
		t.Fatalf("结果丢失: %+v", res.Results)
	}

	raw["heartbeat"] = uint8(1)
	raw["results"] = nil
	b, err = msgpack.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err = UnmarshalResults(b)
	if err != nil {
		t.Fatalf("ping 回包解码失败: %v", err)
	}
	if !res.IsHeartbeat() {
		t.Fatal("heartbeat=1 应视为心跳")
	}
}

func TestUnmarshalResults_heartbeatString(t *testing.T) {
	raw := map[string]interface{}{
		"error": "", "status": false, "heartbeat": "pong", "is_only": false,
	}
	b, err := msgpack.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := UnmarshalResults(b)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsHeartbeat() {
		t.Fatal("heartbeat=pong 应视为心跳")
	}
}

func asInt(v interface{}) (int, bool) {
	switch n := v.(type) {
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint8:
		return int(n), true
	case uint16:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	default:
		return 0, false
	}
}

func TestUnmarshalResults(t *testing.T) {
	// 构造一条与 Yearning queryResults 同构的 msgpack。
	raw := map[string]interface{}{
		"export":     false,
		"error":      "",
		"query_time": 12,
		"status":     false,
		"heartbeat":  "",
		"is_only":    false,
		"results": []interface{}{
			map[string]interface{}{
				"field": []interface{}{
					map[string]interface{}{"title": "id", "dataIndex": "id"},
					map[string]interface{}{"title": "name", "dataIndex": "name"},
				},
				"data": []interface{}{
					map[string]interface{}{"id": "1", "name": "alice"},
					map[string]interface{}{"id": "2", "name": "bob"},
				},
			},
		},
	}
	b, err := msgpack.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	res, err := UnmarshalResults(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("results 数量 = %d, 期望 1", len(res.Results))
	}
	q := res.Results[0]
	if len(q.Field) != 2 || len(q.Data) != 2 {
		t.Fatalf("field=%d data=%d, 期望 2/2", len(q.Field), len(q.Data))
	}
	if q.Field[0]["title"] != "id" || q.Data[0]["name"] != "alice" {
		t.Fatalf("解析结果不符: %v / %v", q.Field[0], q.Data[0])
	}
}
