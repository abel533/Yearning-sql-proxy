package yearning

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	mpv5 "github.com/vmihailenco/msgpack/v5"

	"github.com/toolsHelp/Yearning-sql-proxy/internal/msgpack"
)

// newTestConn 起一个本地 WebSocket 服务端并建立一条测试连接。
func newTestConn(t *testing.T) *gorillaConn {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	gc, err := dial(url, "", srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("dial 失败: %v", err)
	}
	t.Cleanup(gc.close)
	return gc
}

func deadlineNow() time.Time { return time.Now().Add(time.Second) }

// ping 失败后会话必须被标记为失效，否则会留下「IsDead() 为 false 但读写都失败」
// 的僵尸会话，让后续查询反复报 websocket: close sent。
func TestQuerySessionKeepAliveMarksDead(t *testing.T) {
	gc := newTestConn(t)
	qs := &QuerySession{srcID: "src-1", conn: gc}

	if qs.IsDead() {
		t.Fatalf("新建会话不应为失效状态")
	}
	// 模拟 ping 失败路径：keepAlive 会调用 conn.close()。
	gc.close()
	if !qs.IsDead() {
		t.Fatalf("关闭后 IsDead() 应为 true")
	}
	// 已关闭的连接读取应返回 errClosed（归属连接类错误），而不是通用错误。
	if _, _, err := gc.read(deadlineNow()); !errors.Is(err, errClosed) {
		t.Fatalf("已关闭连接的 read 应返回 errClosed, got %v", err)
	}
	if !IsConnError(errClosed) {
		t.Errorf("errClosed 应被 IsConnError 识别")
	}
}

// 3.1.2 只接受 msgpack 控制帧：打开连接是 type=2，心跳是 type=3。
// 文本 ping 会被服务端解码失败后直接断开。
func TestOpenAndPingAreBinaryControlFrames(t *testing.T) {
	var mu sync.Mutex
	type frame struct {
		kind int
		data []byte
	}
	var got []frame
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		ws, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer ws.Close()
		for {
			kind, data, err := ws.ReadMessage()
			if err != nil {
				return
			}
			mu.Lock()
			got = append(got, frame{kind: kind, data: append([]byte(nil), data...)})
			mu.Unlock()
		}
	}))
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	gc, err := dial(url, "tok", srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("dial 失败: %v", err)
	}
	t.Cleanup(gc.close)
	qs := &QuerySession{srcID: "src-1", conn: gc}
	if err := qs.open(); err != nil {
		t.Fatal(err)
	}
	if err := qs.ping(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) < 2 {
		t.Fatalf("收到 %d 帧，期望至少 2", len(got))
	}
	types := []int{msgpack.TypeOpen, msgpack.TypePing}
	for i, want := range types {
		if got[i].kind != websocket.BinaryMessage {
			t.Errorf("第 %d 帧应为二进制，实际 %d", i, got[i].kind)
		}
		var m map[string]interface{}
		if err := mpv5.Unmarshal(got[i].data, &m); err != nil {
			t.Fatal(err)
		}
		gotType, ok := asInt64(m["type"])
		if !ok || gotType != want {
			t.Errorf("第 %d 帧 type=%v，期望 %d", i, m["type"], want)
		}
	}
}

func asInt64(v interface{}) (int, bool) {
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
	case uint64:
		return int(n), true
	default:
		return 0, false
	}
}

func TestIsConnError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errClosed, true},
		{errors.New("查询等待超时"), false},
		{errors.New("websocket: close sent"), true},
		{errors.New("use of closed network connection"), true},
		{errors.New("connection reset by peer"), true},
	}
	for _, c := range cases {
		if got := IsConnError(c.err); got != c.want {
			t.Errorf("IsConnError(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// wrappedClosedError 模拟上层用 %w 包装后的连接错误。
type wrappedClosedError struct{ inner error }

func (w wrappedClosedError) Error() string { return "重连后查询仍失败: " + w.inner.Error() }
func (w wrappedClosedError) Unwrap() error { return w.inner }

func TestIsConnErrorWrapped(t *testing.T) {
	err := wrappedClosedError{inner: errClosed}
	if !IsConnError(err) {
		t.Errorf("包装后的连接错误应被识别: %v", err)
	}
}
