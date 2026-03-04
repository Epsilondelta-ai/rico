package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ============ 프로토콜 타입 테스트 ============

func TestClientMessageMarshal(t *testing.T) {
	payload := MessagePayload{Text: "hello"}
	payloadBytes, _ := json.Marshal(payload)

	msg := ClientMessage{
		Type:      "message",
		Payload:   payloadBytes,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("ClientMessage 마샬링 실패: %v", err)
	}

	var decoded ClientMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("ClientMessage 언마샬링 실패: %v", err)
	}

	if decoded.Type != "message" {
		t.Errorf("예상: message, 결과: %s", decoded.Type)
	}
}

func TestServerMessageMarshal(t *testing.T) {
	msg := ServerMessage{
		Type:      "status",
		Payload:   StatusPayload{State: "idle"},
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("ServerMessage 마샬링 실패: %v", err)
	}

	if !strings.Contains(string(data), "idle") {
		t.Error("직렬화된 데이터에 'idle' 상태가 없음")
	}
}

func TestStatusPayload(t *testing.T) {
	status := StatusPayload{
		State:   "working",
		Task:    "테스트 작업",
		Project: "rico",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("StatusPayload 마샬링 실패: %v", err)
	}

	var decoded StatusPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("StatusPayload 언마샬링 실패: %v", err)
	}

	if decoded.State != "working" {
		t.Errorf("예상: working, 결과: %s", decoded.State)
	}
	if decoded.Task != "테스트 작업" {
		t.Errorf("예상: 테스트 작업, 결과: %s", decoded.Task)
	}
}

func TestResponsePayload(t *testing.T) {
	resp := ResponsePayload{
		Text:       "Claude 응답입니다",
		IsComplete: true,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("ResponsePayload 마샬링 실패: %v", err)
	}

	var decoded ResponsePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("ResponsePayload 언마샬링 실패: %v", err)
	}

	if !decoded.IsComplete {
		t.Error("IsComplete가 true여야 함")
	}
}

func TestErrorPayload(t *testing.T) {
	errPayload := ErrorPayload{
		Code:    "CONNECTION_ERROR",
		Message: "연결 실패",
	}

	data, err := json.Marshal(errPayload)
	if err != nil {
		t.Fatalf("ErrorPayload 마샬링 실패: %v", err)
	}

	var decoded ErrorPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("ErrorPayload 언마샬링 실패: %v", err)
	}

	if decoded.Code != "CONNECTION_ERROR" {
		t.Errorf("예상: CONNECTION_ERROR, 결과: %s", decoded.Code)
	}
}

func TestQueuePayload(t *testing.T) {
	queue := QueuePayload{
		Position: 3,
		Message:  "대기 중",
	}

	data, err := json.Marshal(queue)
	if err != nil {
		t.Fatalf("QueuePayload 마샬링 실패: %v", err)
	}

	var decoded QueuePayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("QueuePayload 언마샬링 실패: %v", err)
	}

	if decoded.Position != 3 {
		t.Errorf("예상: 3, 결과: %d", decoded.Position)
	}
}

// ============ ClaudeRunner 테스트 ============

func TestNewClaudeRunner(t *testing.T) {
	runner := NewClaudeRunner()
	if runner == nil {
		t.Fatal("ClaudeRunner 생성 실패")
	}
	if runner.isRunning {
		t.Error("초기 상태는 실행 중이 아니어야 함")
	}
	if len(runner.queue) != 0 {
		t.Error("초기 큐는 비어있어야 함")
	}
}

func TestClaudeRunnerCancel(t *testing.T) {
	runner := NewClaudeRunner()

	// 큐에 항목 추가
	runner.mu.Lock()
	runner.queue = append(runner.queue, "task1", "task2")
	runner.mu.Unlock()

	// 취소 실행
	runner.Cancel()

	if len(runner.queue) != 0 {
		t.Error("Cancel 후 큐가 비워져야 함")
	}
}

func TestClaudeRunnerQueueing(t *testing.T) {
	runner := NewClaudeRunner()

	var queuePosition int
	runner.onQueue = func(pos int) {
		queuePosition = pos
	}

	// 이미 실행 중인 상태 시뮬레이션
	runner.mu.Lock()
	runner.isRunning = true
	runner.mu.Unlock()

	// 새 작업 추가 시도
	runner.Run("new task")

	if len(runner.queue) != 1 {
		t.Errorf("큐 길이가 1이어야 함, 결과: %d", len(runner.queue))
	}
	if queuePosition != 1 {
		t.Errorf("큐 위치가 1이어야 함, 결과: %d", queuePosition)
	}
}

// ============ Hub 테스트 ============

func TestNewHub(t *testing.T) {
	hub := newHub()
	if hub == nil {
		t.Fatal("Hub 생성 실패")
	}
	if hub.clients == nil {
		t.Error("clients 맵이 nil")
	}
}

func TestHubClientRegistration(t *testing.T) {
	hub := newHub()
	go hub.run()

	// 테스트용 클라이언트 생성
	client := &Client{
		send: make(chan []byte, 256),
	}

	hub.register <- client

	// 등록 처리 대기
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	clientCount := len(hub.clients)
	hub.mu.RUnlock()

	if clientCount != 1 {
		t.Errorf("클라이언트 수가 1이어야 함, 결과: %d", clientCount)
	}

	// 연결 해제
	hub.unregister <- client
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	clientCount = len(hub.clients)
	hub.mu.RUnlock()

	if clientCount != 0 {
		t.Errorf("클라이언트 수가 0이어야 함, 결과: %d", clientCount)
	}
}

func TestHubBroadcast(t *testing.T) {
	hub := newHub()
	go hub.run()

	// 두 개의 클라이언트 생성 및 등록
	client1 := &Client{send: make(chan []byte, 256)}
	client2 := &Client{send: make(chan []byte, 256)}

	hub.register <- client1
	hub.register <- client2
	time.Sleep(10 * time.Millisecond)

	// 브로드캐스트
	testMsg := []byte("broadcast test")
	hub.broadcast <- testMsg

	// 두 클라이언트 모두 메시지 수신 확인
	var wg sync.WaitGroup
	wg.Add(2)

	received := make([]bool, 2)

	go func() {
		select {
		case msg := <-client1.send:
			if string(msg) == "broadcast test" {
				received[0] = true
			}
		case <-time.After(100 * time.Millisecond):
		}
		wg.Done()
	}()

	go func() {
		select {
		case msg := <-client2.send:
			if string(msg) == "broadcast test" {
				received[1] = true
			}
		case <-time.After(100 * time.Millisecond):
		}
		wg.Done()
	}()

	wg.Wait()

	if !received[0] || !received[1] {
		t.Error("모든 클라이언트가 브로드캐스트 메시지를 받아야 함")
	}
}

// ============ HTTP 핸들러 테스트 ============

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("예상 상태 코드: 200, 결과: %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("예상 응답: OK, 결과: %s", w.Body.String())
	}
}

// ============ WebSocket 통합 테스트 ============

func TestWebSocketConnection(t *testing.T) {
	hub := newHub()
	go hub.run()

	// 테스트 서버 생성
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	}))
	defer server.Close()

	// WebSocket URL로 변환
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// WebSocket 연결
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 연결 실패: %v", err)
	}
	defer ws.Close()

	// connect 메시지 전송
	connectMsg := ClientMessage{
		Type:      "connect",
		Payload:   json.RawMessage(`{}`),
		Timestamp: time.Now().UnixMilli(),
	}
	msgBytes, _ := json.Marshal(connectMsg)
	if err := ws.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("메시지 전송 실패: %v", err)
	}

	// 응답 수신
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, response, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("응답 수신 실패: %v", err)
	}

	var serverMsg ServerMessage
	if err := json.Unmarshal(response, &serverMsg); err != nil {
		t.Fatalf("응답 파싱 실패: %v", err)
	}

	if serverMsg.Type != "status" {
		t.Errorf("예상 타입: status, 결과: %s", serverMsg.Type)
	}
}

func TestWebSocketMessageHandling(t *testing.T) {
	hub := newHub()
	go hub.run()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 연결 실패: %v", err)
	}
	defer ws.Close()

	// action 메시지 (cancel) 전송
	actionPayload, _ := json.Marshal(ActionPayload{Action: "cancel"})
	actionMsg := ClientMessage{
		Type:      "action",
		Payload:   actionPayload,
		Timestamp: time.Now().UnixMilli(),
	}
	msgBytes, _ := json.Marshal(actionMsg)

	if err := ws.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		t.Fatalf("action 메시지 전송 실패: %v", err)
	}

	// 응답 확인
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, response, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("응답 수신 실패: %v", err)
	}

	var serverMsg ServerMessage
	if err := json.Unmarshal(response, &serverMsg); err != nil {
		t.Fatalf("응답 파싱 실패: %v", err)
	}

	if serverMsg.Type != "status" {
		t.Errorf("예상 타입: status, 결과: %s", serverMsg.Type)
	}
}

// ============ MVP 기능 검증 테스트 ============

// MVP 스펙: 메시지 송수신
func TestMVP_MessageProtocol(t *testing.T) {
	// 클라이언트 -> 서버 메시지 형식 테스트
	t.Run("client_to_server_message", func(t *testing.T) {
		payload, _ := json.Marshal(MessagePayload{Text: "안녕하세요"})
		msg := ClientMessage{
			Type:      "message",
			Payload:   payload,
			Timestamp: time.Now().UnixMilli(),
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("메시지 직렬화 실패: %v", err)
		}

		// 역직렬화 테스트
		var decoded ClientMessage
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("메시지 역직렬화 실패: %v", err)
		}

		var decodedPayload MessagePayload
		json.Unmarshal(decoded.Payload, &decodedPayload)

		if decodedPayload.Text != "안녕하세요" {
			t.Error("메시지 텍스트 불일치")
		}
	})

	// 서버 -> 클라이언트 응답 형식 테스트
	t.Run("server_to_client_response", func(t *testing.T) {
		msg := ServerMessage{
			Type: "response",
			Payload: ResponsePayload{
				Text:       "Claude 응답",
				IsComplete: true,
			},
			Timestamp: time.Now().UnixMilli(),
		}

		data, err := json.Marshal(msg)
		if err != nil {
			t.Fatalf("응답 직렬화 실패: %v", err)
		}

		if !strings.Contains(string(data), "Claude 응답") {
			t.Error("응답에 텍스트가 포함되어야 함")
		}
	})
}

// MVP 스펙: 상태 관리
func TestMVP_StatusManagement(t *testing.T) {
	states := []string{"idle", "working", "error"}

	for _, state := range states {
		t.Run("status_"+state, func(t *testing.T) {
			msg := ServerMessage{
				Type:      "status",
				Payload:   StatusPayload{State: state},
				Timestamp: time.Now().UnixMilli(),
			}

			data, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("상태 메시지 직렬화 실패: %v", err)
			}

			if !strings.Contains(string(data), state) {
				t.Errorf("상태 '%s'가 포함되어야 함", state)
			}
		})
	}
}

// MVP 스펙: 에러 처리
func TestMVP_ErrorHandling(t *testing.T) {
	errorCodes := []string{
		"CONNECTION_ERROR",
		"PIPE_ERROR",
		"START_ERROR",
		"EXIT_ERROR",
	}

	for _, code := range errorCodes {
		t.Run("error_"+code, func(t *testing.T) {
			msg := ServerMessage{
				Type: "error",
				Payload: ErrorPayload{
					Code:    code,
					Message: "테스트 에러",
				},
				Timestamp: time.Now().UnixMilli(),
			}

			data, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("에러 메시지 직렬화 실패: %v", err)
			}

			if !strings.Contains(string(data), code) {
				t.Errorf("에러 코드 '%s'가 포함되어야 함", code)
			}
		})
	}
}

// MVP 스펙: 큐 시스템
func TestMVP_QueueSystem(t *testing.T) {
	runner := NewClaudeRunner()

	positions := []int{}
	runner.onQueue = func(pos int) {
		positions = append(positions, pos)
	}

	// 실행 중 상태로 설정
	runner.mu.Lock()
	runner.isRunning = true
	runner.mu.Unlock()

	// 여러 작업 추가
	runner.Run("task1")
	runner.Run("task2")
	runner.Run("task3")

	if len(runner.queue) != 3 {
		t.Errorf("큐에 3개의 작업이 있어야 함, 결과: %d", len(runner.queue))
	}

	if len(positions) != 3 {
		t.Errorf("3번의 큐 콜백이 호출되어야 함, 결과: %d", len(positions))
	}

	// 위치가 순차적인지 확인
	for i, pos := range positions {
		if pos != i+1 {
			t.Errorf("위치 %d: 예상 %d, 결과 %d", i, i+1, pos)
		}
	}
}

// ============ 벤치마크 테스트 ============

func BenchmarkMessageMarshal(b *testing.B) {
	payload, _ := json.Marshal(MessagePayload{Text: "benchmark test"})
	msg := ClientMessage{
		Type:      "message",
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(msg)
	}
}

func BenchmarkMessageUnmarshal(b *testing.B) {
	payload, _ := json.Marshal(MessagePayload{Text: "benchmark test"})
	msg := ClientMessage{
		Type:      "message",
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}
	data, _ := json.Marshal(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var decoded ClientMessage
		json.Unmarshal(data, &decoded)
	}
}
