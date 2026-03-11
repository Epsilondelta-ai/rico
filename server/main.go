package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
)

// 환경변수에서 로드되는 설정
var (
	VAPID_PUBLIC_KEY  string
	VAPID_PRIVATE_KEY string
	RICO_BASE_PATH    string
	SERVER_PORT       string
	SSL_CERT_FILE     string
	SSL_KEY_FILE      string
)

// 기본 경로 (server 폴더) - 환경변수에서 로드
var BASE_PATH string

// 환경변수 로드 및 초기화
func init() {
	// .env 파일 로드 (없어도 에러 아님)
	godotenv.Load()

	// 환경변수에서 설정 로드 (기본값 제공)
	VAPID_PUBLIC_KEY = getEnv("VAPID_PUBLIC_KEY", "")
	VAPID_PRIVATE_KEY = getEnv("VAPID_PRIVATE_KEY", "")

	// 경로 설정 - 환경변수 없으면 실행 파일 기준으로 자동 감지
	RICO_BASE_PATH = getEnv("RICO_BASE_PATH", "")
	if RICO_BASE_PATH == "" {
		// 실행 파일 위치 기준으로 rico 폴더 찾기
		execPath, err := os.Executable()
		if err == nil {
			RICO_BASE_PATH = filepath.Dir(filepath.Dir(execPath))
		} else {
			// 현재 작업 디렉토리 사용
			RICO_BASE_PATH, _ = os.Getwd()
			RICO_BASE_PATH = filepath.Dir(RICO_BASE_PATH)
		}
	}

	BASE_PATH = getEnv("SERVER_BASE_PATH", filepath.Join(RICO_BASE_PATH, "server"))
	SERVER_PORT = getEnv("SERVER_PORT", "8081")
	SSL_CERT_FILE = getEnv("SSL_CERT_FILE", "")
	SSL_KEY_FILE = getEnv("SSL_KEY_FILE", "")

	log.Printf("Rico 설정 로드 완료:")
	log.Printf("  - RICO_BASE_PATH: %s", RICO_BASE_PATH)
	log.Printf("  - BASE_PATH: %s", BASE_PATH)
	log.Printf("  - SERVER_PORT: %s", SERVER_PORT)
	log.Printf("  - VAPID_PUBLIC_KEY: %s...", truncate(VAPID_PUBLIC_KEY, 20))
}

// 환경변수 가져오기 (기본값 지원)
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// 문자열 자르기
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// 파일명에 사용할 수 없는 문자 제거
func sanitizeFileName(name string) string {
	// 파일명에 사용할 수 없는 문자들을 언더스코어로 대체
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := name
	for _, ch := range invalid {
		result = strings.ReplaceAll(result, ch, "_")
	}
	// 공백도 언더스코어로
	result = strings.ReplaceAll(result, " ", "_")
	// 너무 길면 자르기 (최대 30자)
	if len(result) > 30 {
		result = result[:30]
	}
	return result
}

// 세션 로그 파일명 생성 (날짜_제목_ID.log)
func getSessionLogFileName(sessionID string) string {
	sessionLogDir := filepath.Join(BASE_PATH, "logs", "sessions")

	// 세션 정보 가져오기
	if sessionStore != nil {
		session := sessionStore.GetSession(sessionID)
		if session != nil && session.Title != "" {
			// 세션 생성 날짜 (UpdatedAt 기준, 없으면 현재 시간)
			var dateStr string
			if session.UpdatedAt > 0 {
				dateStr = time.UnixMilli(session.UpdatedAt).Format("2006-01-02")
			} else {
				dateStr = time.Now().Format("2006-01-02")
			}

			// 날짜_제목_ID.log 형식
			safeTitle := sanitizeFileName(session.Title)
			return filepath.Join(sessionLogDir, fmt.Sprintf("%s_%s_%s.log", dateStr, safeTitle, sessionID))
		}
	}

	// 세션 정보 없으면 기존 방식 (ID만)
	return filepath.Join(sessionLogDir, sessionID+".log")
}

// 세션별 로그 기록
func logSession(sessionID, format string, args ...interface{}) {
	if sessionID == "" {
		return
	}

	// 세션 로그 폴더 생성
	sessionLogDir := filepath.Join(BASE_PATH, "logs", "sessions")
	os.MkdirAll(sessionLogDir, 0755)

	// 세션별 로그 파일 (날짜_제목_ID.log)
	logFileName := getSessionLogFileName(sessionID)
	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[세션로그 에러] 파일 열기 실패: %s - %v", logFileName, err)
		return
	}
	defer f.Close()

	// 타임스탬프 + 메시지
	timestamp := time.Now().Format("2006/01/02 15:04:05")
	message := fmt.Sprintf(format, args...)
	bytesWritten, writeErr := fmt.Fprintf(f, "[%s] %s\n", timestamp, message)
	if writeErr != nil {
		log.Printf("[세션로그 에러] 쓰기 실패: %s - %v", logFileName, writeErr)
	} else {
		log.Printf("[세션로그] 성공: %s (%d bytes)", logFileName, bytesWritten)
	}
}

// CLAUDECODE 환경 변수 제거된 환경 반환 (중첩 세션 방지)
func getCleanEnvForClaude() []string {
	var cleanEnv []string
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "CLAUDECODE=") {
			cleanEnv = append(cleanEnv, env)
		}
	}
	return cleanEnv
}

// 푸시 구독 저장소
var pushSubscriptions = make(map[string]*webpush.Subscription)
var pushMu sync.RWMutex
var pushSubscriptionsFile = "push_subscriptions.json"

// ============ 로그 스트리밍 ============

type LogEntry struct {
	Timestamp int64  `json:"timestamp"`
	Level     string `json:"level"`   // "info", "error", "warn"
	Message   string `json:"message"`
	Source    string `json:"source"`  // "go", "vite", "system"
}

type LogBuffer struct {
	entries     []LogEntry
	maxSize     int
	mu          sync.RWMutex
	subscribers map[chan LogEntry]bool
	subMu       sync.RWMutex
}

var serverLogBuffer = &LogBuffer{
	entries:     make([]LogEntry, 0, 500),
	maxSize:     500,
	subscribers: make(map[chan LogEntry]bool),
}

// 로그 추가 및 구독자에게 브로드캐스트
func (lb *LogBuffer) Add(level, message, source string) {
	entry := LogEntry{
		Timestamp: time.Now().UnixMilli(),
		Level:     level,
		Message:   message,
		Source:    source,
	}

	lb.mu.Lock()
	if len(lb.entries) >= lb.maxSize {
		lb.entries = lb.entries[1:]
	}
	lb.entries = append(lb.entries, entry)
	lb.mu.Unlock()

	// 구독자들에게 브로드캐스트 (비동기)
	lb.subMu.RLock()
	for ch := range lb.subscribers {
		select {
		case ch <- entry:
		default:
			// 채널이 가득 차면 스킵
		}
	}
	lb.subMu.RUnlock()
}

// 최근 로그 조회
func (lb *LogBuffer) GetRecent(count int) []LogEntry {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if count <= 0 || count > len(lb.entries) {
		count = len(lb.entries)
	}
	start := len(lb.entries) - count
	result := make([]LogEntry, count)
	copy(result, lb.entries[start:])
	return result
}

// 구독 시작
func (lb *LogBuffer) Subscribe() chan LogEntry {
	ch := make(chan LogEntry, 50)
	lb.subMu.Lock()
	lb.subscribers[ch] = true
	lb.subMu.Unlock()
	return ch
}

// 구독 해제
func (lb *LogBuffer) Unsubscribe(ch chan LogEntry) {
	lb.subMu.Lock()
	delete(lb.subscribers, ch)
	lb.subMu.Unlock()
	close(ch)
}

// 커스텀 로그 Writer (log.Printf를 로그 버퍼로 복제)
type LogWriter struct {
	source string
}

func (w *LogWriter) Write(p []byte) (n int, err error) {
	message := strings.TrimSpace(string(p))
	if message == "" {
		return len(p), nil
	}

	// 로그 레벨 감지
	level := "info"
	if strings.Contains(message, "Error") || strings.Contains(message, "에러") || strings.Contains(message, "실패") {
		level = "error"
	} else if strings.Contains(message, "Warning") || strings.Contains(message, "경고") {
		level = "warn"
	}

	serverLogBuffer.Add(level, message, w.source)
	return len(p), nil
}

// ============ 설정 (Settings) ============

type RicoSettings struct {
	Modes struct {
	} `json:"modes"`
}

var ricoSettings RicoSettings
var settingsPath = "settings.json"
var settingsMu sync.RWMutex

// ============ SOUL 생성 상태 ============

type SoulGenerateStatus struct {
	Status  string `json:"status"` // "generating", "done", "error"
	Content string `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

var soulGenerateResult SoulGenerateStatus
var soulGenerateMu sync.RWMutex

// 설정 파일 로드
func loadSettings() {
	filePath := filepath.Join(ricoBasePath, settingsPath)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("설정 파일 읽기 실패 (기본값 사용): %v", err)
		return
	}

	settingsMu.Lock()
	defer settingsMu.Unlock()
	if err := json.Unmarshal(data, &ricoSettings); err != nil {
		log.Printf("설정 파일 파싱 실패: %v", err)
		return
	}
	log.Printf("설정 로드 완료")
}

// 설정 파일 저장
func saveSettings() {
	settingsMu.RLock()
	data, err := json.MarshalIndent(ricoSettings, "", "  ")
	settingsMu.RUnlock()

	if err != nil {
		log.Printf("설정 직렬화 실패: %v", err)
		return
	}

	filePath := filepath.Join(ricoBasePath, settingsPath)
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("설정 파일 저장 실패: %v", err)
		return
	}
	log.Printf("설정 저장 완료")
}

// 활성 페르소나 이름 가져오기 (전역)
func getActivePersonaName() string {
	activePersonaPath := filepath.Join(ricoBasePath, "context", "personas", "active.json")
	data, err := ioutil.ReadFile(activePersonaPath)
	if err != nil {
		return "default"
	}
	var active struct {
		Current string `json:"current"`
	}
	if json.Unmarshal(data, &active) != nil || active.Current == "" {
		return "default"
	}
	return active.Current
}

// SOUL 프롬프트 파일 경로 반환 (새 personas 구조 사용)
func getSoulPromptFile() (string, func()) {
	activePersona := getActivePersonaName()
	soulPath := filepath.Join(ricoBasePath, "context", "personas", activePersona, "SOUL.md")
	_, err := ioutil.ReadFile(soulPath)
	if err != nil {
		log.Printf("SOUL.md 읽기 실패 (%s): %v", activePersona, err)
		return "", nil
	}

	return soulPath, nil
}

// 푸시 구독 파일에서 로드
func loadPushSubscriptions() {
	filePath := filepath.Join(ricoBasePath, pushSubscriptionsFile)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		// 파일이 없으면 무시
		return
	}

	var subs map[string]*webpush.Subscription
	if err := json.Unmarshal(data, &subs); err != nil {
		log.Printf("푸시 구독 파일 파싱 실패: %v", err)
		return
	}

	pushMu.Lock()
	pushSubscriptions = subs
	pushMu.Unlock()
	log.Printf("푸시 구독 %d개 로드됨", len(subs))
}

// 푸시 구독 파일에 저장
func savePushSubscriptions() {
	pushMu.RLock()
	data, err := json.MarshalIndent(pushSubscriptions, "", "  ")
	pushMu.RUnlock()

	if err != nil {
		log.Printf("푸시 구독 직렬화 실패: %v", err)
		return
	}

	filePath := filepath.Join(ricoBasePath, pushSubscriptionsFile)
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("푸시 구독 파일 저장 실패: %v", err)
		return
	}
}

// ============ 키워드 트리거 (Context Guard) ============

// 키워드 -> 컨텍스트 파일 매핑
var keywordTriggers = map[string]string{
	// memory-management.md
	"기억해":   "memory-management.md",
	"잊지마":   "memory-management.md",
	"기억해둬":  "memory-management.md",

	// file-sharing.md
	"이미지":      "file-sharing.md",
	"이미지 올려":  "file-sharing.md",
	"파일 보내":   "file-sharing.md",
	"첨부해":     "file-sharing.md",
	"jpg":       "file-sharing.md",
	"png":       "file-sharing.md",
	"gif":       "file-sharing.md",
	"스크린샷":   "file-sharing.md",

	// qna-recording.md
	"Q&A 기록": "qna-recording.md",
	"기록해줘":  "qna-recording.md",
	"이해했어":  "qna-recording.md",

	// interactive-commands.md
	"서버 시작":  "interactive-commands.md",
	"npm run":   "interactive-commands.md",
	"go run":    "interactive-commands.md",
	"빌드해":    "interactive-commands.md",

	// DEV_GUIDE.md
	"구현해줘":   "DEV_GUIDE.md",
	"코드 작성":  "DEV_GUIDE.md",
	"개발해줘":   "DEV_GUIDE.md",
	"리팩토링":   "DEV_GUIDE.md",

	// screens/README.md
	"화면 구조":  "screens/README.md",
	"UI 구조":   "screens/README.md",

	// architecture/README.md
	"API 목록":   "architecture/README.md",
	"아키텍처":   "architecture/README.md",
}

// Rico 프로젝트 경로 (환경변수에서 로드)
var ricoBasePath string
var contextBasePath string

func initPaths() {
	ricoBasePath = RICO_BASE_PATH
	contextBasePath = filepath.Join(RICO_BASE_PATH, "context")
}

// CLAUDE.md 내용 캐시
var systemPromptCache string

// CLAUDE.md 로드
func loadSystemPrompt() string {
	if systemPromptCache != "" {
		return systemPromptCache
	}

	claudeMdPath := filepath.Join(ricoBasePath, "CLAUDE.md")
	content, err := ioutil.ReadFile(claudeMdPath)
	if err != nil {
		log.Printf("CLAUDE.md 읽기 실패: %v", err)
		return ""
	}
	systemPromptCache = string(content)
	log.Printf("CLAUDE.md 로드 완료 (%d bytes)", len(systemPromptCache))
	return systemPromptCache
}

// 메시지에서 키워드 감지 후 해당 컨텍스트 파일들 반환 (중복 제거)
func detectKeywordTriggers(message string) []string {
	matchedFiles := make(map[string]bool)
	lowerMsg := strings.ToLower(message)

	for keyword, file := range keywordTriggers {
		if strings.Contains(lowerMsg, strings.ToLower(keyword)) {
			matchedFiles[file] = true
		}
	}

	// map을 slice로 변환
	var files []string
	for file := range matchedFiles {
		files = append(files, file)
	}
	return files
}

// 컨텍스트 파일 내용 읽기
func readContextFile(filename string) string {
	filePath := filepath.Join(contextBasePath, filename)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("컨텍스트 파일 읽기 실패: %s - %v", filename, err)
		return ""
	}
	return string(content)
}

// 메시지에 컨텍스트 Read 지시 주입 (키워드 트리거)
// 파일 내용을 직접 주입하지 않고, Read 지시만 추가하여 캐시 중복 방지
func injectContext(message string) string {
	files := detectKeywordTriggers(message)
	if len(files) == 0 {
		return message
	}

	log.Printf("키워드 트리거 발동: %v", files)

	// Read 지시 생성 (파일 내용 대신 경로만 전달)
	var readInstructions []string
	for _, file := range files {
		filePath := filepath.Join(contextBasePath, file)
		readInstructions = append(readInstructions, fmt.Sprintf("- %s", filePath))
	}

	// 컨텍스트 Read 지시를 메시지 앞에 붙임
	contextHeader := fmt.Sprintf(`-
---
[컨텍스트 참조 지시]
다음 파일들을 Read 도구로 읽고 규칙을 따르세요:
%s

---
[유저 메시지]
`, strings.Join(readInstructions, "\n"))

	return contextHeader + message
}

// SOUL 컨텐츠 정리 (앞뒤 불필요한 텍스트 제거)
func cleanSoulContent(content string) string {
	// "# SOUL" 찾기
	idx := strings.Index(content, "# SOUL")
	if idx == -1 {
		// 못 찾으면 "# "로 시작하는 부분 찾기
		idx = strings.Index(content, "\n# ")
		if idx != -1 {
			idx++ // \n 다음부터
		}
	}
	if idx == -1 {
		// 그래도 못 찾으면 원본 그대로 반환
		return strings.TrimSpace(content)
	}
	return strings.TrimSpace(content[idx:])
}

// ============ 프로토콜 타입 정의 ============

type ClientMessage struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp int64           `json:"timestamp"`
}

type MessagePayload struct {
	Text      string `json:"text"`
	SessionID string `json:"sessionId,omitempty"`
}

type ActionPayload struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params,omitempty"`
}

type ServerMessage struct {
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload"`
	Timestamp int64       `json:"timestamp"`
}

type StatusPayload struct {
	State        string   `json:"state"`
	Task         string   `json:"task,omitempty"`
	Project      string   `json:"project,omitempty"`
	PendingTools []string `json:"pendingTools,omitempty"` // 진행 중인 도구 목록 (재연결 시 복원용)
}

// 토큰 사용량 정보
type TokenUsage struct {
	InputTokens              int     `json:"inputTokens"`
	OutputTokens             int     `json:"outputTokens"`
	CacheCreationInputTokens int     `json:"cacheCreationInputTokens,omitempty"`
	CacheReadInputTokens     int     `json:"cacheReadInputTokens,omitempty"`
	TotalCostUSD             float64 `json:"totalCostUsd,omitempty"`
}

type ResponsePayload struct {
	Text        string       `json:"text"`
	IsComplete  bool         `json:"isComplete"`
	Suggestions []string     `json:"suggestions,omitempty"`
	ToolsUsed   []string     `json:"toolsUsed,omitempty"`   // 사용된 도구 목록 (실시간)
	ToolDetails []ToolDetail `json:"toolDetails,omitempty"` // 도구 상세 정보
	TokenUsage  *TokenUsage  `json:"tokenUsage,omitempty"`  // 토큰 사용량
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type QueuePayload struct {
	Position int    `json:"position"`
	Message  string `json:"message,omitempty"`
}

// ============ 도구 인스펙터 ============

// 도구 사용 상세 정보
type ToolDetail struct {
	Name        string   `json:"name"`                  // "Read", "Edit", "Grep" 등
	File        string   `json:"file,omitempty"`        // 파일 경로
	Offset      int      `json:"offset,omitempty"`      // 시작 줄 (Read)
	Limit       int      `json:"limit,omitempty"`       // 줄 수 (Read)
	Line        int      `json:"line,omitempty"`        // 수정 줄 번호 (Edit, 대략적)
	OldString   string   `json:"oldString,omitempty"`   // 변경 전 문자열 (Edit)
	NewString   string   `json:"newString,omitempty"`   // 변경 후 문자열 (Edit)
	Pattern     string   `json:"pattern,omitempty"`     // 검색 패턴 (Grep)
	Command     string   `json:"command,omitempty"`     // 실행 명령어 (Bash)
	Suggestions []string `json:"suggestions,omitempty"` // 제안 (StructuredOutput)
}

// tool_use input에서 파일 경로 추출
func extractFilePathFromInput(input interface{}) string {
	if input == nil {
		return ""
	}
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return ""
	}

	// 다양한 도구에서 사용하는 파일 경로 필드들
	pathFields := []string{"file_path", "path", "pattern", "command", "query"}
	for _, field := range pathFields {
		if val, ok := inputMap[field].(string); ok && val != "" {
			return val
		}
	}
	return ""
}

// stream-json 이벤트에서 도구 이름과 파일경로로 상태 메시지 생성
func getToolStatusMessage(toolName string, filePath string) string {
	// 파일 경로에서 파일명만 추출
	fileName := filePath
	if filePath != "" {
		// 경로에서 파일명만 추출 (마지막 / 또는 \ 이후)
		for i := len(filePath) - 1; i >= 0; i-- {
			if filePath[i] == '/' || filePath[i] == '\\' {
				fileName = filePath[i+1:]
				break
			}
		}
	}

	// 파일명이 있으면 "도구: 파일명" 형식으로
	if fileName != "" {
		return toolName + ": " + fileName
	}

	// 파일명이 없으면 도구 이름만
	return toolName
}

// tool_use input에서 상세 정보 추출
func extractToolDetail(toolName string, input interface{}) ToolDetail {
	detail := ToolDetail{Name: toolName}

	if input == nil {
		return detail
	}

	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return detail
	}

	// 파일 경로 추출
	pathFields := []string{"file_path", "path", "notebook_path"}
	for _, field := range pathFields {
		if val, ok := inputMap[field].(string); ok && val != "" {
			detail.File = val
			break
		}
	}

	// Read 도구: offset, limit 추출
	if toolName == "Read" {
		if offset, ok := inputMap["offset"].(float64); ok {
			detail.Offset = int(offset)
		}
		if limit, ok := inputMap["limit"].(float64); ok {
			detail.Limit = int(limit)
		}
	}

	// Edit 도구: old_string, new_string 추출
	if toolName == "Edit" {
		if oldStr, ok := inputMap["old_string"].(string); ok {
			detail.OldString = oldStr
		}
		if newStr, ok := inputMap["new_string"].(string); ok {
			detail.NewString = newStr
		}
	}

	// Grep 도구: pattern 저장
	if toolName == "Grep" {
		if pattern, ok := inputMap["pattern"].(string); ok {
			detail.Pattern = pattern
		}
	}

	// Bash 도구: command 저장
	if toolName == "Bash" {
		if cmd, ok := inputMap["command"].(string); ok {
			detail.Command = cmd
		}
	}

	// Glob 도구: pattern 저장
	if toolName == "Glob" {
		if pattern, ok := inputMap["pattern"].(string); ok {
			detail.Pattern = pattern
		}
	}

	// StructuredOutput 도구: suggestions 저장
	if toolName == "StructuredOutput" {
		if suggestionsRaw, ok := inputMap["suggestions"].([]interface{}); ok {
			for _, s := range suggestionsRaw {
				if str, ok := s.(string); ok {
					detail.Suggestions = append(detail.Suggestions, str)
				}
			}
		}
	}

	return detail
}

// ============ 세션/메시지 저장소 ============

// Claude CLI JSON 응답 구조체
type ClaudeResponse struct {
	Type             string  `json:"type"`
	Result           string  `json:"result"`
	SessionID        string  `json:"session_id"`
	IsError          bool    `json:"is_error"`
	TotalCostUSD     float64 `json:"total_cost_usd,omitempty"`
	Usage            *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage,omitempty"`
	StructuredOutput *struct {
		Response    string   `json:"response"`
		Suggestions []string `json:"suggestions,omitempty"`
	} `json:"structured_output,omitempty"`
}

type ChatMessage struct {
	ID           string       `json:"id"`
	Text         string       `json:"text"`
	IsUser       bool         `json:"isUser"`
	Timestamp    int64        `json:"timestamp"`
	IsSystem     bool         `json:"isSystem,omitempty"`     // 시스템 메시지 (세션 복원 시 필터링)
	ToolsUsed    []string     `json:"toolsUsed,omitempty"`    // 사용된 도구 목록 (예: "Read: main.go") - 호환용
	ToolDetails  []ToolDetail `json:"toolDetails,omitempty"`  // 도구 상세 정보
	OutputTokens int          `json:"outputTokens,omitempty"` // 출력 토큰 수
}

type Session struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Messages        []ChatMessage `json:"messages"`
	UpdatedAt       int64         `json:"updatedAt"`
	ClaudeSessionID string        `json:"claudeSessionId,omitempty"` // Claude Code 세션 ID
	LastSuggestions []string      `json:"lastSuggestions,omitempty"` // 마지막 응답의 suggestions
}

type SessionStore struct {
	Sessions map[string]*Session `json:"sessions"`
	mu       sync.RWMutex
	filePath string
}

func NewSessionStore(filePath string) *SessionStore {
	store := &SessionStore{
		Sessions: make(map[string]*Session),
		filePath: filePath,
	}
	store.load()
	return store
}

func (s *SessionStore) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := ioutil.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("세션 파일 로드 실패: %v", err)
		}
		return
	}

	var sessions map[string]*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		log.Printf("세션 파싱 실패: %v", err)
		return
	}

	s.Sessions = sessions
	log.Printf("세션 로드 완료: %d개", len(s.Sessions))
}

func (s *SessionStore) save() {
	data, err := json.MarshalIndent(s.Sessions, "", "  ")
	if err != nil {
		log.Printf("세션 직렬화 실패: %v", err)
		return
	}

	// 디렉토리 생성
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("디렉토리 생성 실패: %v", err)
		return
	}

	if err := ioutil.WriteFile(s.filePath, data, 0644); err != nil {
		log.Printf("세션 저장 실패: %v", err)
	}
}

func (s *SessionStore) GetSession(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Sessions[id]
}

func (s *SessionStore) GetAllSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.Sessions))
	for _, session := range s.Sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (s *SessionStore) CreateSession(id, title string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 타이틀이 "새 대화"면 세션 ID 앞 8자로 대체
	if title == "새 대화" && len(id) >= 8 {
		title = id[:8]
	}

	session := &Session{
		ID:        id,
		Title:     title,
		Messages:  []ChatMessage{},
		UpdatedAt: time.Now().UnixMilli(),
	}
	s.Sessions[id] = session
	s.save()
	return session
}

func (s *SessionStore) AddMessage(sessionID string, msg ChatMessage) {
	s.mu.Lock()

	session, exists := s.Sessions[sessionID]
	isFirstUserMessage := false

	if !exists {
		// 세션 없으면 생성 (임시 타이틀)
		session = &Session{
			ID:       sessionID,
			Title:    "새 대화",
			Messages: []ChatMessage{},
		}
		s.Sessions[sessionID] = session
		isFirstUserMessage = msg.IsUser
	} else if msg.IsUser && len(session.Messages) == 0 {
		isFirstUserMessage = true
	}

	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now().UnixMilli()
	s.save()
	s.mu.Unlock()

	// 첫 유저 메시지면 비동기로 제목 요약 요청
	if isFirstUserMessage {
		go s.generateTitle(sessionID, msg.Text)
	}
}

// Claude 세션 ID 업데이트
func (s *SessionStore) UpdateClaudeSessionID(ricoSessionID, claudeSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.Sessions[ricoSessionID]
	if exists {
		session.ClaudeSessionID = claudeSessionID
		s.save()
	}
}

func (s *SessionStore) UpdateLastSuggestions(sessionID string, suggestions []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, exists := s.Sessions[sessionID]
	if exists {
		session.LastSuggestions = suggestions
		s.save()
	}
}

// Claude에게 제목 요약 요청 (비동기, Haiku 모델 사용)
func (s *SessionStore) generateTitle(sessionID, firstMessage string) {
	log.Printf("제목 생성 시작: %s", firstMessage)

	// 간결한 프롬프트 - 제목만 출력하도록 강제
	prompt := `[TASK] 아래 메시지의 의도를 한국어 3~5단어로 요약해서 제목만 출력하라. 다른 말 하지마.

[MESSAGE] ` + firstMessage + `

[TITLE]`

	// Haiku 모델 + 세션 저장 안 함 + 짧은 응답
	cmd := exec.Command("claude",
		"--print",
		"--model", "haiku",
		"--no-session-persistence",
		prompt)
	cmd.Env = getCleanEnvForClaude() // CLAUDECODE 환경 변수 제거

	output, err := cmd.Output()
	if err != nil {
		log.Printf("제목 생성 실패: %v", err)
		return
	}

	title := strings.TrimSpace(string(output))
	if title == "" {
		return
	}

	// [TITLE] 접두어 제거 (혹시 포함되면)
	title = strings.TrimPrefix(title, "[TITLE]")
	title = strings.TrimSpace(title)

	// 요약 실패 판단: 너무 길거나 설명투로 시작하면 원본 메시지 사용
	failedPrefixes := []string{"작업", "메시지", "요청", "한국어", "알겠", "네,", "안녕"}
	isFailed := len(title) > 25
	for _, prefix := range failedPrefixes {
		if strings.HasPrefix(title, prefix) {
			isFailed = true
			break
		}
	}

	if isFailed {
		// 원본 메시지 앞 20글자를 제목으로 (룬 단위)
		runes := []rune(firstMessage)
		if len(runes) > 20 {
			title = string(runes[:20]) + "..."
		} else {
			title = firstMessage
		}
	}

	log.Printf("제목 생성 완료: %s -> %s", firstMessage, title)

	// 세션 제목 업데이트
	s.mu.Lock()
	if session, exists := s.Sessions[sessionID]; exists {
		session.Title = title
		s.save()
	}
	s.mu.Unlock()

	// 기존 로그 파일(ID만)을 새 이름(날짜_제목_ID)으로 리네임
	sessionLogDir := filepath.Join(BASE_PATH, "logs", "sessions")
	oldLogFile := filepath.Join(sessionLogDir, sessionID+".log")
	if _, err := os.Stat(oldLogFile); err == nil {
		// 기존 파일이 존재하면 리네임
		dateStr := time.Now().Format("2006-01-02")
		safeTitle := sanitizeFileName(title)
		newLogFile := filepath.Join(sessionLogDir, fmt.Sprintf("%s_%s_%s.log", dateStr, safeTitle, sessionID))
		if err := os.Rename(oldLogFile, newLogFile); err != nil {
			log.Printf("로그 파일 리네임 실패: %s -> %s: %v", oldLogFile, newLogFile, err)
		} else {
			log.Printf("로그 파일 리네임 완료: %s -> %s", oldLogFile, newLogFile)
		}
	}
}

func (s *SessionStore) DeleteSession(id string) {
	s.mu.Lock()

	// 삭제 전에 로그 파일명 가져오기 (세션 정보가 있을 때)
	var sessionLogFile string
	if session, exists := s.Sessions[id]; exists && session.Title != "" {
		var dateStr string
		if session.UpdatedAt > 0 {
			dateStr = time.UnixMilli(session.UpdatedAt).Format("2006-01-02")
		} else {
			dateStr = time.Now().Format("2006-01-02")
		}
		safeTitle := sanitizeFileName(session.Title)
		sessionLogFile = filepath.Join(BASE_PATH, "logs", "sessions", fmt.Sprintf("%s_%s_%s.log", dateStr, safeTitle, id))
	} else {
		// 세션 정보 없으면 ID만으로 시도
		sessionLogFile = filepath.Join(BASE_PATH, "logs", "sessions", id+".log")
	}

	delete(s.Sessions, id)
	s.save()
	s.mu.Unlock()

	// 세션 로그 파일 삭제
	if err := os.Remove(sessionLogFile); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("세션 로그 삭제 실패: %s - %v", sessionLogFile, err)
		}
	} else {
		log.Printf("세션 로그 삭제 완료: %s", sessionLogFile)
	}
}

// 전역 세션 저장소
var sessionStore *SessionStore

// 세션별 작업 상태 (working/idle)
var sessionWorkingState = make(map[string]bool)
var sessionCurrentTask = make(map[string]string)    // 현재 진행 중인 도구 상태
var sessionPendingTools = make(map[string][]string) // 진행 중인 도구 목록 전체
var sessionPendingToolsSet = make(map[string]map[string]bool) // 중복 방지용
var sessionWorkingMu sync.RWMutex

func setSessionWorking(sessionID string, working bool) {
	sessionWorkingMu.Lock()
	defer sessionWorkingMu.Unlock()
	if working {
		sessionWorkingState[sessionID] = true
	} else {
		delete(sessionWorkingState, sessionID)
		delete(sessionCurrentTask, sessionID)
		delete(sessionPendingTools, sessionID)
		delete(sessionPendingToolsSet, sessionID)
	}
}

func setSessionCurrentTask(sessionID string, task string) {
	sessionWorkingMu.Lock()
	defer sessionWorkingMu.Unlock()
	sessionCurrentTask[sessionID] = task
}

func addSessionPendingTool(sessionID string, tool string) {
	sessionWorkingMu.Lock()
	defer sessionWorkingMu.Unlock()
	// 중복 방지
	if sessionPendingToolsSet[sessionID] == nil {
		sessionPendingToolsSet[sessionID] = make(map[string]bool)
	}
	if !sessionPendingToolsSet[sessionID][tool] {
		sessionPendingToolsSet[sessionID][tool] = true
		sessionPendingTools[sessionID] = append(sessionPendingTools[sessionID], tool)
	}
}

func getSessionCurrentTask(sessionID string) string {
	sessionWorkingMu.RLock()
	defer sessionWorkingMu.RUnlock()
	return sessionCurrentTask[sessionID]
}

func getSessionPendingTools(sessionID string) []string {
	sessionWorkingMu.RLock()
	defer sessionWorkingMu.RUnlock()
	if tools, ok := sessionPendingTools[sessionID]; ok {
		result := make([]string, len(tools))
		copy(result, tools)
		return result
	}
	return nil
}

func isSessionWorking(sessionID string) bool {
	sessionWorkingMu.RLock()
	defer sessionWorkingMu.RUnlock()
	return sessionWorkingState[sessionID]
}

// ============ Claude Code 세션 읽기 ============

type ClaudeSession struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UpdatedAt int64  `json:"updatedAt"`
	Project   string `json:"project"`
}

type ClaudeMessage struct {
	Type    string `json:"type"`
	Message struct {
		Role    string `json:"role"`
		Content any    `json:"content"` // string 또는 []ContentBlock
	} `json:"message"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	Cwd       string `json:"cwd"`
}

// Claude 세션 디렉토리 경로
func getClaudeProjectsDir() string {
	usr, err := user.Current()
	if err != nil {
		log.Printf("유저 디렉토리 가져오기 실패: %v", err)
		return ""
	}
	return filepath.Join(usr.HomeDir, ".claude", "projects")
}

// 모든 Claude 세션 목록 가져오기
func getClaudeSessions() []ClaudeSession {
	projectsDir := getClaudeProjectsDir()
	log.Printf("Claude 프로젝트 디렉토리: %s", projectsDir)
	if projectsDir == "" {
		return nil
	}

	var sessions []ClaudeSession

	// 프로젝트 디렉토리 순회
	projectDirs, err := ioutil.ReadDir(projectsDir)
	if err != nil {
		log.Printf("프로젝트 디렉토리 읽기 실패: %v", err)
		return nil
	}
	log.Printf("프로젝트 수: %d", len(projectDirs))

	for _, projectDir := range projectDirs {
		if !projectDir.IsDir() {
			log.Printf("스킵 (파일): %s", projectDir.Name())
			continue
		}

		projectPath := filepath.Join(projectsDir, projectDir.Name())
		files, err := ioutil.ReadDir(projectPath)
		if err != nil {
			log.Printf("프로젝트 읽기 실패: %s - %v", projectPath, err)
			continue
		}
		log.Printf("프로젝트 %s: %d개 파일", projectDir.Name(), len(files))

		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}
			log.Printf("  세션 파일 발견: %s", file.Name())

			sessionID := strings.TrimSuffix(file.Name(), ".jsonl")
			filePath := filepath.Join(projectPath, file.Name())

			// 첫 번째 유저 메시지로 제목 추출
			title := getSessionTitle(filePath)
			if title == "" {
				title = "대화 " + sessionID[:8]
			}

			sessions = append(sessions, ClaudeSession{
				ID:        sessionID,
				Title:     title,
				UpdatedAt: file.ModTime().UnixMilli(),
				Project:   projectDir.Name(),
			})
		}
	}

	log.Printf("총 세션 수: %d", len(sessions))

	// 최신순 정렬
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})

	// 최근 20개만 반환
	if len(sessions) > 20 {
		sessions = sessions[:20]
	}

	log.Printf("반환할 세션 수: %d", len(sessions))
	return sessions
}

// 세션 파일에서 제목 추출 (첫 유저 메시지)
func getSessionTitle(filePath string) string {
	file, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 큰 줄 처리를 위해 버퍼 크기 늘림
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var msg ClaudeMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Type == "user" && msg.Message.Role == "user" {
			// content가 string인 경우
			if content, ok := msg.Message.Content.(string); ok {
				title := strings.TrimSpace(content)
				// "-\n" 접두어 제거 (디스코드 봇 포맷)
				title = strings.TrimPrefix(title, "-\n")
				title = strings.TrimSpace(title)
				if len(title) > 50 {
					title = title[:50] + "..."
				}
				return title
			}
		}
	}

	return ""
}

// 특정 세션의 메시지 가져오기
func getClaudeSessionMessages(sessionID string) []ChatMessage {
	projectsDir := getClaudeProjectsDir()
	if projectsDir == "" {
		return nil
	}

	// 세션 파일 찾기
	var filePath string
	projectDirs, _ := ioutil.ReadDir(projectsDir)
	for _, projectDir := range projectDirs {
		if !projectDir.IsDir() {
			continue
		}
		testPath := filepath.Join(projectsDir, projectDir.Name(), sessionID+".jsonl")
		if _, err := os.Stat(testPath); err == nil {
			filePath = testPath
			break
		}
	}

	if filePath == "" {
		return nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var messages []ChatMessage
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		var msg ClaudeMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Type == "user" && msg.Message.Role == "user" {
			if content, ok := msg.Message.Content.(string); ok {
				text := strings.TrimSpace(content)
				text = strings.TrimPrefix(text, "-\n")
				text = strings.TrimSpace(text)
				messages = append(messages, ChatMessage{
					ID:        msg.Timestamp,
					Text:      text,
					IsUser:    true,
					Timestamp: parseTimestamp(msg.Timestamp),
				})
			}
		} else if msg.Type == "assistant" && msg.Message.Role == "assistant" {
			// assistant content는 배열일 수 있음
			text := extractAssistantText(msg.Message.Content)
			if text != "" {
				messages = append(messages, ChatMessage{
					ID:        msg.Timestamp,
					Text:      text,
					IsUser:    false,
					Timestamp: parseTimestamp(msg.Timestamp),
				})
			}
		}
	}

	return messages
}

func extractAssistantText(content any) string {
	// string인 경우
	if s, ok := content.(string); ok {
		return s
	}

	// []interface{} 인 경우 (content blocks)
	if arr, ok := content.([]interface{}); ok {
		var texts []string
		for _, item := range arr {
			if block, ok := item.(map[string]interface{}); ok {
				if block["type"] == "text" {
					if text, ok := block["text"].(string); ok {
						texts = append(texts, text)
					}
				}
			}
		}
		return strings.Join(texts, "\n")
	}

	return ""
}

func parseTimestamp(ts string) int64 {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Now().UnixMilli()
	}
	return t.UnixMilli()
}

// ============ Claude Code 실행기 (큐 방식) ============

type QueueItem struct {
	Prompt    string
	SessionID string
}

type ClaudeRunner struct {
	cmd                  *exec.Cmd
	isRunning            bool
	mu                   sync.Mutex
	queue                []QueueItem
	onOutput             func(text string, isComplete bool, suggestions []string, toolsUsed []string, toolDetails []ToolDetail, tokenUsage *TokenUsage)
	onStatus             func(state, task string)
	onError              func(code, message string)
	onQueue              func(position int)                          // 큐 위치 알림
	onSessionID          func(ricoSessionID, claudeSessionID string) // Claude 세션 ID 콜백
	sessionStore         *SessionStore                               // 세션 저장소 참조
	isPersonaRecharging  bool                                        // 페르소나 충전 중 플래그 (알림 스킵용)
}

func NewClaudeRunner() *ClaudeRunner {
	return &ClaudeRunner{
		queue: make([]QueueItem, 0),
	}
}

func (cr *ClaudeRunner) Run(prompt string, sessionID string) error {
	cr.mu.Lock()
	if cr.isRunning {
		// 큐에 추가
		cr.queue = append(cr.queue, QueueItem{Prompt: prompt, SessionID: sessionID})
		position := len(cr.queue)
		cr.mu.Unlock()
		log.Printf("큐에 추가됨 (위치: %d): %s", position, prompt)
		if cr.onQueue != nil {
			cr.onQueue(position)
		}
		return nil
	}
	cr.isRunning = true
	cr.mu.Unlock()

	cr.execute(prompt, sessionID)
	return nil
}

func (cr *ClaudeRunner) execute(prompt string, ricoSessionID string) {
	// 페르소나 충전 여부 확인
	cr.isPersonaRecharging = strings.Contains(prompt, "[페르소나 충전 - 전체 SOUL 재주입]")

	// 세션 상태를 working으로 설정
	if ricoSessionID != "" {
		setSessionWorking(ricoSessionID, true)
	}

	if cr.onStatus != nil {
		cr.onStatus("working", prompt)
	}

	// Rico 세션에서 Claude 세션 ID 가져오기
	var claudeSessionID string
	if cr.sessionStore != nil && ricoSessionID != "" {
		session := cr.sessionStore.GetSession(ricoSessionID)
		if session != nil && session.ClaudeSessionID != "" {
			claudeSessionID = session.ClaudeSessionID
			log.Printf("[세션] 기존 Claude 세션 이어가기: %s", claudeSessionID)
		} else {
			log.Printf("[세션] 새 Claude 세션 시작 (Rico세션: %s)", ricoSessionID)
		}
	}

	// Claude Code CLI 실행 (--print --verbose --output-format stream-json 모드)
	log.Printf("Claude 실행 시작 (Rico세션: %s, Claude세션: %s): %s", ricoSessionID, claudeSessionID, prompt)

	// SOUL 프롬프트 파일 가져오기
	soulPromptFile, soulCleanup := getSoulPromptFile()

	var cmd *exec.Cmd
	args := []string{"--print", "--verbose", "--output-format", "stream-json"}

	// 세션 이어가기
	if claudeSessionID != "" {
		args = append(args, "--resume", claudeSessionID)
	}

	args = append(args, "--dangerously-skip-permissions")

	// SOUL 프롬프트 파일 추가 (새 세션일 때만 - resume 시에는 이미 기억에 있음)
	if soulPromptFile != "" && claudeSessionID == "" {
		args = append(args, "--append-system-prompt-file", soulPromptFile)
		log.Printf("SOUL 프롬프트 파일 적용: %s", soulPromptFile)
	} else if claudeSessionID != "" {
		log.Printf("세션 이어가기 - SOUL 프롬프트 생략 (이미 기억에 있음)")
	}

	// JSON 스키마 추가
	schema := `{"type":"object","properties":{"response":{"type":"string"},"suggestions":{"type":"array","items":{"type":"string"},"minItems":3,"maxItems":3}},"required":["response","suggestions"]}`
	log.Printf("JSON 스키마 적용")
	args = append(args, "--json-schema", schema)

	// stdin으로 prompt 전달 (-p - 옵션 사용)
	args = append(args, "-p", "-")
	cmd = exec.Command("claude", args...)
	cmd.Env = getCleanEnvForClaude() // CLAUDECODE 환경 변수 제거
	cr.cmd = cmd

	stdin, err := cr.cmd.StdinPipe()
	if err != nil {
		cr.handleError("PIPE_ERROR", err.Error())
		cr.processNext()
		return
	}

	stdout, err := cr.cmd.StdoutPipe()
	if err != nil {
		cr.handleError("PIPE_ERROR", err.Error())
		cr.processNext()
		return
	}

	stderr, err := cr.cmd.StderrPipe()
	if err != nil {
		cr.handleError("PIPE_ERROR", err.Error())
		cr.processNext()
		return
	}

	if err := cr.cmd.Start(); err != nil {
		cr.handleError("START_ERROR", err.Error())
		cr.processNext()
		return
	}

	// stdin으로 prompt 전달 후 닫기
	go func() {
		defer stdin.Close()
		stdin.Write([]byte(prompt))
	}()

	// stream-json 모드: 실시간 이벤트 파싱
	go func() {
		var lastResultJSON string // 마지막 result 이벤트의 JSON
		var toolsUsed []string
		var toolDetails []ToolDetail
		toolsUsedMap := make(map[string]bool)
		isFirstAssistant := true // 첫 번째 assistant 이벤트 플래그 (--resume 히스토리 무시용)

		// stderr 읽기 (로그용) - stdout과 동시에 시작해야 cmd.Wait()이 빨리 끝남
		go func() {
			errScanner := bufio.NewScanner(stderr)
			for errScanner.Scan() {
				line := errScanner.Text()
				log.Printf("Claude stderr: %s", line)
			}
		}()

		// stdout 읽기 - stream-json 이벤트 처리
		scanner := bufio.NewScanner(stdout)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// JSON 이벤트 파싱
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				log.Printf("stream-json 파싱 실패: %v, line: %s", err, line)
				continue
			}

			eventType, _ := event["type"].(string)

			// 이벤트 타입별 처리
			switch eventType {
			case "assistant":
				// 도구 사용 이벤트
				// --resume 시 히스토리도 assistant 이벤트로 오지만,
				// content_block_start에서 리셋하므로 여기서는 누적만 함
				if message, ok := event["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].([]interface{}); ok {
						for _, c := range content {
							if block, ok := c.(map[string]interface{}); ok {
								if block["type"] == "tool_use" {
									toolName, _ := block["name"].(string)
									filePath := extractFilePathFromInput(block["input"])
									if toolName != "" {
										status := getToolStatusMessage(toolName, filePath)
										// 중복 방지하여 도구 목록에 추가
										if !toolsUsedMap[status] {
											toolsUsedMap[status] = true
											toolsUsed = append(toolsUsed, status)
											// 상세 정보도 추가
											detail := extractToolDetail(toolName, block["input"])
											toolDetails = append(toolDetails, detail)
										}
										if cr.onStatus != nil {
											cr.onStatus("working", status)
										}
									}
								}
							}
						}
					}
				}

			case "content_block_start":
				// 콘텐츠 블록 시작 (tool_use 포함)
				// 참고: content_block_start는 현재 응답에서만 옴 (히스토리에서는 안 옴)
				// 첫 content_block_start가 오면 히스토리에서 누적된 도구들 리셋
				if isFirstAssistant {
					toolsUsed = nil
					toolDetails = nil
					toolsUsedMap = make(map[string]bool)
					isFirstAssistant = false
				}

				if contentBlock, ok := event["content_block"].(map[string]interface{}); ok {
					if contentBlock["type"] == "tool_use" {
						toolName, _ := contentBlock["name"].(string)
						if cr.onStatus != nil && toolName != "" {
							status := getToolStatusMessage(toolName, "")
							cr.onStatus("working", status)
						}
					}
				}

			case "result":
				// 최종 결과 이벤트 - JSON 저장
				lastResultJSON = line
			}
		}

		// 완료 대기
		err := cr.cmd.Wait()
		log.Printf("Claude 완료. 도구 사용: %d개", len(toolsUsed))

		if err != nil {
			log.Printf("Claude 에러: %v", err)
			if cr.onError != nil {
				cr.onError("EXIT_ERROR", err.Error())
			}
		}

		// stream-json의 result 이벤트에서 응답 추출
		if lastResultJSON == "" {
			log.Printf("result 이벤트 없음")
			// 세션 상태 idle로 복원
			if ricoSessionID != "" {
				setSessionWorking(ricoSessionID, false)
			}
			if cr.onStatus != nil {
				cr.onStatus("idle", "")
			}
			cr.processNext()
			return
		}

		// result 이벤트를 ClaudeResponse로 파싱
		var claudeResp ClaudeResponse
		responseText := lastResultJSON

		if err := json.Unmarshal([]byte(responseText), &claudeResp); err != nil {
			log.Printf("JSON 파싱 실패, 원본 텍스트 사용: %v", err)
			// JSON 파싱 실패시 원본 텍스트 그대로 사용
			if cr.onOutput != nil && len(responseText) > 0 {
				cr.onOutput(responseText, true, nil, toolsUsed, toolDetails, nil)
			}
			// JSON 파싱 실패해도 세션에 저장
			if cr.sessionStore != nil && ricoSessionID != "" && responseText != "" {
				botMsg := ChatMessage{
					ID:          time.Now().Format("20060102150405") + "_bot",
					Text:        responseText,
					IsUser:      false,
					Timestamp:   time.Now().UnixMilli(),
					IsSystem:    false,
					ToolsUsed:   toolsUsed,
					ToolDetails: toolDetails,
				}
				cr.sessionStore.AddMessage(ricoSessionID, botMsg)
				log.Printf("JSON 파싱 실패 응답도 세션 저장 완료 (세션: %s)", ricoSessionID)
				logSession(ricoSessionID, "[CLAUDE] %s", responseText)
			}
		} else {
			// JSON 파싱 성공
			log.Printf("Claude 응답 파싱 성공. 세션ID: %s, Result길이: %d", claudeResp.SessionID, len(claudeResp.Result))
			log.Printf("Claude Result 내용: %s", claudeResp.Result)

			// Claude 세션 ID 저장
			if claudeResp.SessionID != "" && cr.sessionStore != nil && ricoSessionID != "" {
				cr.sessionStore.UpdateClaudeSessionID(ricoSessionID, claudeResp.SessionID)
				log.Printf("Claude 세션 ID 저장됨: %s -> %s", ricoSessionID, claudeResp.SessionID)
			}

			// 응답 텍스트 처리
			var finalResponse string
			var suggestions []string

			// structured_output이 있으면 (JSON 스키마 사용 시)
			if claudeResp.StructuredOutput != nil {
				finalResponse = claudeResp.StructuredOutput.Response
				suggestions = claudeResp.StructuredOutput.Suggestions
				if len(suggestions) > 0 {
					log.Printf("[제안] suggestions=%v", suggestions)
					// StructuredOutput toolDetail에 suggestions 추가
					for i := range toolDetails {
						if toolDetails[i].Name == "StructuredOutput" {
							toolDetails[i].Suggestions = suggestions
							break
						}
					}
				}
			} else {
				// 기존 방식 (result 필드) - JSON 스키마 없이 실행된 경우
				finalResponse = claudeResp.Result
			}

			// 토큰 사용량 정보 추출 (세션 저장 전에 먼저 추출)
			var tokenUsage *TokenUsage
			var outputTokens int
			if claudeResp.Usage != nil {
				outputTokens = claudeResp.Usage.OutputTokens
				tokenUsage = &TokenUsage{
					InputTokens:              claudeResp.Usage.InputTokens,
					OutputTokens:             outputTokens,
					CacheCreationInputTokens: claudeResp.Usage.CacheCreationInputTokens,
					CacheReadInputTokens:     claudeResp.Usage.CacheReadInputTokens,
					TotalCostUSD:             claudeResp.TotalCostUSD,
				}
				log.Printf("📊 [토큰] 입력: %d, 출력: %d, 캐시생성: %d, 캐시읽기: %d, 비용: $%.4f",
					tokenUsage.InputTokens, tokenUsage.OutputTokens,
					tokenUsage.CacheCreationInputTokens, tokenUsage.CacheReadInputTokens,
					tokenUsage.TotalCostUSD)
			}

			// Claude 응답을 세션에 저장 (ricoSessionID는 execute 파라미터로 캡처됨)
			if cr.sessionStore != nil && ricoSessionID != "" && finalResponse != "" {
				// 페르소나 충전 응답인지 확인 (원본 프롬프트 기반)
				isPersonaRecharge := strings.Contains(prompt, "[페르소나 충전 - 전체 SOUL 재주입]")

				if isPersonaRecharge {
					// 페르소나 충전: Claude 응답은 isSystem: true로 숨기고, "충전 완료" 메시지는 isSystem: false로 표시
					botMsg := ChatMessage{
						ID:          time.Now().Format("20060102150405") + "_bot",
						Text:        finalResponse,
						IsUser:      false,
						Timestamp:   time.Now().UnixMilli(),
						IsSystem:    true, // Claude 응답은 숨김
						ToolsUsed:   toolsUsed,
						ToolDetails: toolDetails,
					}
					cr.sessionStore.AddMessage(ricoSessionID, botMsg)

					// "페르소나 충전 완료" 메시지 추가 (이건 보이게)
					completeMsg := ChatMessage{
						ID:        time.Now().Format("20060102150405") + "_complete",
						Text:      "🔋 페르소나 충전 완료",
						IsUser:    false,
						Timestamp: time.Now().UnixMilli(),
						IsSystem:  false, // 이건 보임
					}
					cr.sessionStore.AddMessage(ricoSessionID, completeMsg)
				} else {
					// 일반 응답
					botMsg := ChatMessage{
						ID:           time.Now().Format("20060102150405") + "_bot",
						Text:         finalResponse,
						IsUser:       false,
						Timestamp:    time.Now().UnixMilli(),
						IsSystem:     false,
						ToolsUsed:    toolsUsed,
						ToolDetails:  toolDetails,
						OutputTokens: outputTokens, // 출력 토큰 수 저장
					}
					cr.sessionStore.AddMessage(ricoSessionID, botMsg)
				}
				log.Printf("응답 세션 저장 완료 (세션: %s), 도구: %v", ricoSessionID, toolsUsed)
				logSession(ricoSessionID, "[CLAUDE_RAW] %s", responseText)
				logSession(ricoSessionID, "[CLAUDE] %s", finalResponse)

				// suggestions도 세션에 저장
				if len(suggestions) > 0 {
					cr.sessionStore.UpdateLastSuggestions(ricoSessionID, suggestions)
					log.Printf("suggestions 저장 완료 (세션: %s): %v", ricoSessionID, suggestions)
				}
			}

			// 응답 텍스트 전송 (WebSocket으로)
			if cr.onOutput != nil {
				if finalResponse != "" {
					log.Printf("응답 전송: %s", finalResponse[:min(50, len(finalResponse))])
					cr.onOutput(finalResponse, true, suggestions, toolsUsed, toolDetails, tokenUsage)
				} else {
					log.Printf("빈 응답 전송")
					cr.onOutput("", true, nil, toolsUsed, toolDetails, nil)
				}
			}
		}

		// 세션 상태를 idle로 설정
		if ricoSessionID != "" {
			setSessionWorking(ricoSessionID, false)
		}

		// SOUL 임시 파일 정리
		if soulCleanup != nil {
			soulCleanup()
		}

		// 다음 큐 처리
		cr.processNext()
	}()
}

func (cr *ClaudeRunner) processNext() {
	cr.mu.Lock()
	if len(cr.queue) > 0 {
		// 큐에서 다음 항목 꺼내기
		next := cr.queue[0]
		cr.queue = cr.queue[1:]
		cr.mu.Unlock()
		log.Printf("큐에서 다음 처리: %s (남은 큐: %d)", next.Prompt, len(cr.queue))
		cr.execute(next.Prompt, next.SessionID)
	} else {
		cr.isRunning = false
		cr.mu.Unlock()
		if cr.onStatus != nil {
			cr.onStatus("idle", "")
		}
	}
}

func (cr *ClaudeRunner) Cancel() {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	// 큐 비우기
	cr.queue = make([]QueueItem, 0)

	if cr.cmd != nil && cr.cmd.Process != nil {
		pid := cr.cmd.Process.Pid
		log.Printf("Claude 프로세스 취소 시도 (PID: %d)", pid)

		// Windows: taskkill /T /F /PID 로 프로세스 트리 전체 종료
		// Unix: Kill()로 충분하지만, Windows에서는 자식 프로세스가 남을 수 있음
		killCmd := exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprintf("%d", pid))
		if err := killCmd.Run(); err != nil {
			log.Printf("taskkill 실패, 직접 Kill 시도: %v", err)
			cr.cmd.Process.Kill()
		}
		cr.isRunning = false
		log.Printf("Claude 프로세스 취소 완료")
	}
}

func (cr *ClaudeRunner) handleError(code, message string) {
	cr.mu.Lock()
	cr.isRunning = false
	cr.mu.Unlock()

	if cr.onError != nil {
		cr.onError(code, message)
	}
	if cr.onStatus != nil {
		cr.onStatus("error", message)
	}
}

// ============ 푸시 알림 전송 ============

func sendPushNotification(title, body string) {
	pushMu.RLock()
	defer pushMu.RUnlock()

	if len(pushSubscriptions) == 0 {
		log.Println("푸시 구독자 없음")
		return
	}

	// 알림 페이로드 생성
	payload, _ := json.Marshal(map[string]string{
		"title": title,
		"body":  body,
	})

	for endpoint, sub := range pushSubscriptions {
		go func(endpoint string, sub *webpush.Subscription) {
			resp, err := webpush.SendNotification(payload, sub, &webpush.Options{
				Subscriber:      "rico@example.com",
				VAPIDPublicKey:  VAPID_PUBLIC_KEY,
				VAPIDPrivateKey: VAPID_PRIVATE_KEY,
				TTL:             30,
			})
			if err != nil {
				log.Printf("푸시 전송 실패 (%s): %v", endpoint[:30], err)
				return
			}
			defer resp.Body.Close()
			log.Printf("푸시 전송 완료: %d", resp.StatusCode)
		}(endpoint, sub)
	}
}

// ============ WebSocket 서버 ============

// WebSocket heartbeat 설정
const (
	// 클라이언트가 pong을 보내야 하는 제한 시간
	pongWait = 60 * time.Second
	// 서버가 ping을 보내는 주기 (pongWait보다 짧아야 함)
	pingPeriod = 30 * time.Second
	// 쓰기 타임아웃
	writeWait = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn      *websocket.Conn
	send      chan []byte
	runner    *ClaudeRunner
	sessionID string // 현재 세션 ID
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("클라이언트 연결됨. 총:", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				// 연결 끊김 시 Claude 실행을 취소하지 않음
				// Claude는 계속 실행되고, 응답은 세션에 저장됨
				// 명시적 취소 요청(cancel 메시지)이 올 때만 Cancel() 호출
			}
			h.mu.Unlock()
			log.Println("클라이언트 연결 해제 (Claude 계속 실행). 총:", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (c *Client) sendMessage(msg ServerMessage) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("sendMessage 복구: %v (클라이언트 연결 끊김)", r)
		}
	}()

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON 마샬링 실패: %v", err)
		return
	}

	select {
	case c.send <- data:
		// 전송 성공
	default:
		log.Printf("채널 가득 참 또는 닫힘, 메시지 드롭")
	}
}

func (c *Client) handleMessage(raw []byte) {
	var msg ClientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		log.Printf("메시지 파싱 실패: %v", err)
		return
	}

	switch msg.Type {
	case "connect":
		// connect 메시지에서 세션 ID 추출
		var connectPayload MessagePayload
		if err := json.Unmarshal(msg.Payload, &connectPayload); err == nil {
			if connectPayload.SessionID != "" {
				c.sessionID = connectPayload.SessionID
				log.Printf("클라이언트 연결 (세션: %s)", c.sessionID)
			}
		}

		// 전역 세션 상태 확인 후 전송
		state := "idle"
		task := ""
		var pendingTools []string
		if c.sessionID != "" && isSessionWorking(c.sessionID) {
			state = "working"
			task = getSessionCurrentTask(c.sessionID)
			pendingTools = getSessionPendingTools(c.sessionID)
		}
		log.Printf("연결 시 상태 전송: 세션=%s, 상태=%s, task=%s, pendingTools=%d개", c.sessionID, state, task, len(pendingTools))
		c.sendMessage(ServerMessage{
			Type:      "status",
			Payload:   StatusPayload{State: state, Task: task, PendingTools: pendingTools},
			Timestamp: time.Now().UnixMilli(),
		})

	case "message":
		var payload MessagePayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("payload 파싱 실패: %v", err)
			return
		}

		// 세션 ID 설정
		if payload.SessionID != "" {
			c.sessionID = payload.SessionID
		}

		log.Printf("명령 수신 (세션: %s): %s", c.sessionID, payload.Text)
		logSession(c.sessionID, "[USER] %s", payload.Text)

		// 유저 메시지 Rico 세션에 저장
		if c.sessionID != "" {
			// 페르소나 충전 메시지인지 확인
			isSystemMsg := strings.Contains(payload.Text, "[페르소나 충전 - 전체 SOUL 재주입]")
			userMsg := ChatMessage{
				ID:        time.Now().Format("20060102150405") + "_user",
				Text:      payload.Text,
				IsUser:    true,
				Timestamp: time.Now().UnixMilli(),
				IsSystem:  isSystemMsg,
			}
			sessionStore.AddMessage(c.sessionID, userMsg)
		}

		// 키워드 트리거로 컨텍스트 주입
		promptWithContext := injectContext(payload.Text)

		// Claude Code 실행 (Rico 세션 ID 전달 -> Claude 세션 ID 조회/저장)
		c.runner.Run(promptWithContext, c.sessionID)

	case "action":
		var payload ActionPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Printf("payload 파싱 실패: %v", err)
			return
		}
		log.Printf("액션 수신: %s", payload.Action)

		if payload.Action == "cancel" {
			c.runner.Cancel()
			// 세션 상태 정리 (working -> idle)
			if c.sessionID != "" {
				setSessionWorking(c.sessionID, false)
			}
			c.sendMessage(ServerMessage{
				Type:      "status",
				Payload:   StatusPayload{State: "idle", Task: "취소됨"},
				Timestamp: time.Now().UnixMilli(),
			})
		} else if payload.Params != nil && payload.Params["prompt"] != "" {
			c.runner.Run(payload.Params["prompt"], c.sessionID)
		}
	}
}

func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	// pong 타임아웃 설정
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		// pong 받으면 타임아웃 갱신
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("에러: %v", err)
			}
			break
		}
		c.handleMessage(message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 채널이 닫힘
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		case <-ticker.C:
			// 30초마다 ping 전송 (heartbeat)
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket 업그레이드 실패:", err)
		return
	}

	runner := NewClaudeRunner()
	runner.sessionStore = sessionStore // 세션 저장소 연결

	client := &Client{
		conn:   conn,
		send:   make(chan []byte, 256),
		runner: runner,
	}

	runner.onOutput = func(text string, isComplete bool, suggestions []string, toolsUsed []string, toolDetails []ToolDetail, tokenUsage *TokenUsage) {
		// 세션 저장은 execute 내부에서 ricoSessionID로 직접 처리
		// 여기서는 WebSocket 전송만 담당

		client.sendMessage(ServerMessage{
			Type: "response",
			Payload: ResponsePayload{
				Text:        text,
				IsComplete:  isComplete,
				Suggestions: suggestions,
				ToolsUsed:   toolsUsed,
				ToolDetails: toolDetails,
				TokenUsage:  tokenUsage,
			},
			Timestamp: time.Now().UnixMilli(),
		})

		// 응답 완료 시 푸시 알림 전송 (페르소나 충전은 제외)
		if isComplete && text != "" && !runner.isPersonaRecharging {
			// 응답 요약 (50자까지)
			summary := text
			runes := []rune(summary)
			if len(runes) > 50 {
				summary = string(runes[:50]) + "..."
			}
			sendPushNotification("EB 응답 완료", summary)
		}
	}

	runner.onStatus = func(state, task string) {
		// 세션별 현재 태스크 저장 (재연결 시 복원용)
		if client.sessionID != "" && state == "working" {
			setSessionCurrentTask(client.sessionID, task)
			// 도구 사용인 경우 pendingTools에도 추가
			if task != "" && task != "생각하는 중" {
				addSessionPendingTool(client.sessionID, task)
			}
		}
		client.sendMessage(ServerMessage{
			Type: "status",
			Payload: StatusPayload{
				State: state,
				Task:  task,
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}

	runner.onError = func(code, message string) {
		client.sendMessage(ServerMessage{
			Type: "error",
			Payload: ErrorPayload{
				Code:    code,
				Message: message,
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}

	runner.onQueue = func(position int) {
		client.sendMessage(ServerMessage{
			Type: "queue",
			Payload: QueuePayload{
				Position: position,
				Message:  "요청이 큐에 추가되었습니다",
			},
			Timestamp: time.Now().UnixMilli(),
		})
	}

	hub.register <- client

	go client.writePump()
	go client.readPump(hub)
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func main() {
	// 경로 초기화
	initPaths()

	// 로그 폴더 생성
	logsDir := filepath.Join(BASE_PATH, "logs")
	os.MkdirAll(logsDir, 0755)

	// 날짜별 로그 파일 설정 (콘솔 + 파일 + 로그 버퍼 동시 출력)
	logFileName := filepath.Join(logsDir, time.Now().Format("2006-01-02")+".log")
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("로그 파일 생성 실패: %v", err)
	} else {
		// 콘솔, 파일, 로그 버퍼 모두에 로그 출력
		logWriter := &LogWriter{source: "go"}
		multiWriter := io.MultiWriter(os.Stdout, logFile, logWriter)
		log.SetOutput(multiWriter)
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
		log.Printf("=== 서버 시작 === (로그: %s)", logFileName)
		defer logFile.Close()
	}

	// 세션 저장소 초기화
	sessionStore = NewSessionStore(filepath.Join(BASE_PATH, "data", "sessions.json"))

	// 푸시 구독 로드
	loadPushSubscriptions()

	// 설정 로드
	loadSettings()

	hub := newHub()
	go hub.run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 로그 스트리밍 WebSocket
	http.HandleFunc("/ws/logs", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[LogWS] Upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		log.Printf("[LogWS] Client connected")

		// 최근 로그 50개 먼저 전송
		recentLogs := serverLogBuffer.GetRecent(50)
		for _, entry := range recentLogs {
			if err := conn.WriteJSON(entry); err != nil {
				log.Printf("[LogWS] Initial log send failed: %v", err)
				return
			}
		}

		// 실시간 구독
		logChan := serverLogBuffer.Subscribe()
		defer serverLogBuffer.Unsubscribe(logChan)

		// ping/pong 처리
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		// 연결 종료 감지용 goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case entry, ok := <-logChan:
				if !ok {
					return
				}
				if err := conn.WriteJSON(entry); err != nil {
					log.Printf("[LogWS] Send failed: %v", err)
					return
				}
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				log.Printf("[LogWS] Client disconnected")
				return
			}
		}
	})

	// 서버 재시작 API
	http.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("[Restart] Server restart requested (OS: %s)", runtime.GOOS)

		// 응답 먼저 보내기
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Server restart initiated",
			"os":      runtime.GOOS,
		})

		// OS별 빠른 재시작 스크립트 실행 (빌드 없이 서버만 재시작)
		go func() {
			time.Sleep(500 * time.Millisecond) // 응답 전송 대기

			var cmd *exec.Cmd
			scriptsDir := filepath.Join(RICO_BASE_PATH, "scripts")

			switch runtime.GOOS {
			case "windows":
				scriptPath := filepath.Join(scriptsDir, "restart-windows.bat")
				cmd = exec.Command("cmd", "/c", "start", "", scriptPath)
			case "darwin": // macOS
				scriptPath := filepath.Join(scriptsDir, "restart-linux.sh")
				cmd = exec.Command("bash", scriptPath)
			default: // linux
				scriptPath := filepath.Join(scriptsDir, "restart-linux.sh")
				cmd = exec.Command("bash", scriptPath)
			}

			cmd.Dir = RICO_BASE_PATH
			if err := cmd.Start(); err != nil {
				log.Printf("[Restart] Script execution failed: %v", err)
				return
			}

			log.Printf("[Restart] Script executed (%s), shutting down current process", runtime.GOOS)
			os.Exit(0)
		}()
	})

	// Rico 세션 목록 조회
	http.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		sessions := sessionStore.GetAllSessions()

		// 간략한 세션 정보만 반환
		type SessionSummary struct {
			ID           string `json:"id"`
			Title        string `json:"title"`
			LastMessage  string `json:"lastMessage"`
			UpdatedAt    int64  `json:"updatedAt"`
			MessageCount int    `json:"messageCount"`
		}

		summaries := make([]SessionSummary, 0, len(sessions))
		for _, s := range sessions {
			lastMsg := ""
			if len(s.Messages) > 0 {
				lastMsg = s.Messages[len(s.Messages)-1].Text
				runes := []rune(lastMsg)
				if len(runes) > 50 {
					lastMsg = string(runes[:50]) + "..."
				}
			}
			// Claude 응답만 카운트 (유저 메시지, 시스템 메시지 제외)
			claudeMessageCount := 0
			for _, msg := range s.Messages {
				if !msg.IsUser && !msg.IsSystem {
					claudeMessageCount++
				}
			}
			summaries = append(summaries, SessionSummary{
				ID:           s.ID,
				Title:        s.Title,
				LastMessage:  lastMsg,
				UpdatedAt:    s.UpdatedAt,
				MessageCount: claudeMessageCount,
			})
		}

		// 최신순 정렬
		sort.Slice(summaries, func(i, j int) bool {
			return summaries[i].UpdatedAt > summaries[j].UpdatedAt
		})

		json.NewEncoder(w).Encode(summaries)
	})

	// Rico 세션 조회/삭제
	http.HandleFunc("/api/session/", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		sessionID := strings.TrimPrefix(r.URL.Path, "/api/session/")
		if sessionID == "" {
			http.Error(w, "session ID required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if r.Method == "DELETE" {
			sessionStore.DeleteSession(sessionID)
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		session := sessionStore.GetSession(sessionID)
		if session == nil {
			session = sessionStore.CreateSession(sessionID, "새 대화")
		}

		json.NewEncoder(w).Encode(session)
	})

	// 퀵 경로 API (홈, 바탕화면 등 동적 제공)
	http.HandleFunc("/api/quick-paths", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		type QuickPath struct {
			Name string `json:"name"`
			Path string `json:"path"`
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = "/"
		}

		paths := []QuickPath{
			{Name: "Home", Path: homeDir},
		}

		// Windows인 경우 바탕화면 추가
		if runtime.GOOS == "windows" {
			desktopPath := filepath.Join(homeDir, "Desktop")
			if _, err := os.Stat(desktopPath); err == nil {
				paths = append(paths, QuickPath{Name: "Desktop", Path: desktopPath})
			}
		}

		// macOS/Linux인 경우 Documents 추가
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			documentsPath := filepath.Join(homeDir, "Documents")
			if _, err := os.Stat(documentsPath); err == nil {
				paths = append(paths, QuickPath{Name: "Documents", Path: documentsPath})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(paths)
	})

	// 파일 브라우저 API
	http.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		path := r.URL.Query().Get("path")
		if path == "" {
			// 기본 경로: 사용자 홈 디렉토리
			homeDir, err := os.UserHomeDir()
			if err != nil {
				homeDir = "/"
			}
			path = homeDir
		}

		// 경로 읽기
		entries, err := os.ReadDir(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type FileEntry struct {
			Name  string `json:"name"`
			IsDir bool   `json:"isDir"`
		}

		files := make([]FileEntry, 0, len(entries))
		for _, entry := range entries {
			// 숨김 파일 제외 (. 으로 시작)
			if !strings.HasPrefix(entry.Name(), ".") {
				files = append(files, FileEntry{
					Name:  entry.Name(),
					IsDir: entry.IsDir(),
				})
			}
		}

		// 폴더 우선, 그 다음 파일 (각각 이름순)
		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir != files[j].IsDir {
				return files[i].IsDir // 폴더가 먼저
			}
			return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":  path,
			"files": files,
		})
	})

	// 파일 내용 읽기 API (offset/limit 지원)
	http.HandleFunc("/api/file", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		path := r.URL.Query().Get("path")
		if path == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}

		// offset, limit 파라미터 (줄 단위)
		offsetStr := r.URL.Query().Get("offset")
		limitStr := r.URL.Query().Get("limit")
		var offset, limit int
		if offsetStr != "" {
			fmt.Sscanf(offsetStr, "%d", &offset)
		}
		if limitStr != "" {
			fmt.Sscanf(limitStr, "%d", &limit)
		}

		// 허용된 확장자만
		ext := strings.ToLower(filepath.Ext(path))
		allowedExts := map[string]bool{
			".md": true, ".txt": true, ".go": true, ".js": true, ".ts": true,
			".jsx": true, ".tsx": true, ".json": true, ".html": true, ".css": true,
			".svelte": true, ".vue": true, ".py": true, ".rs": true, ".yaml": true,
			".yml": true, ".toml": true, ".sh": true, ".bat": true, ".sql": true,
			".log": true,
		}

		if !allowedExts[ext] {
			http.Error(w, "file type not allowed", http.StatusForbidden)
			return
		}

		// 파일 크기 체크 (1MB 제한)
		info, err := os.Stat(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if info.Size() > 1024*1024 {
			http.Error(w, "file too large", http.StatusForbidden)
			return
		}

		// 파일 읽기
		content, err := ioutil.ReadFile(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// offset/limit이 지정된 경우 줄 단위로 자르기
		finalContent := string(content)
		startLine := 1 // 응답에 포함할 시작 줄 번호
		if offset > 0 || limit > 0 {
			lines := strings.Split(finalContent, "\n")
			totalLines := len(lines)

			// offset은 0-based 인덱스로 사용
			startIdx := offset
			if startIdx < 0 {
				startIdx = 0
			}
			if startIdx >= totalLines {
				finalContent = ""
				startLine = startIdx + 1
			} else {
				endIdx := totalLines
				if limit > 0 {
					endIdx = startIdx + limit
					if endIdx > totalLines {
						endIdx = totalLines
					}
				}
				finalContent = strings.Join(lines[startIdx:endIdx], "\n")
				startLine = startIdx + 1 // 1-based 줄 번호
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"path":      path,
			"name":      filepath.Base(path),
			"ext":       ext,
			"content":   finalContent,
			"startLine": startLine, // 도구 인스펙터용: 줄 번호 시작점
		})
	})

	// Claude Skills 목록 API
	http.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Skills 폴더 경로
		usr, err := user.Current()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		skillsPath := filepath.Join(usr.HomeDir, ".claude", "skills")
		log.Printf("Skills 경로: %s", skillsPath)

		entries, err := os.ReadDir(skillsPath)
		if err != nil {
			log.Printf("Skills 폴더 읽기 실패: %v", err)
			// 폴더가 없으면 빈 배열 반환
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"skills": []string{},
			})
			return
		}

		log.Printf("Skills 폴더 엔트리 수: %d", len(entries))

		type SkillInfo struct {
			Name        string `json:"name"`
			Path        string `json:"path"`
			Description string `json:"description"`
		}

		skills := make([]SkillInfo, 0)
		for _, entry := range entries {
			skillPath := filepath.Join(skillsPath, entry.Name())
			log.Printf("엔트리: %s, IsDir: %v, Type: %v", entry.Name(), entry.IsDir(), entry.Type())

			// 심볼릭 링크 또는 디렉토리 체크
			// Windows에서는 심볼릭 링크가 파일처럼 보일 수 있으므로 os.Stat으로 확인
			info, err := os.Stat(skillPath)
			if err != nil {
				log.Printf("Stat 실패: %s - %v", skillPath, err)
				continue
			}

			if !info.IsDir() {
				log.Printf("스킵 (파일): %s", entry.Name())
				continue
			}

			// 실제 경로 (심볼릭 링크면 해석)
			realPath, err := filepath.EvalSymlinks(skillPath)
			if err != nil {
				log.Printf("심볼릭 링크 해석 실패: %v, 원본 경로 사용", err)
				realPath = skillPath
			}
			log.Printf("실제 경로: %s", realPath)

			// SKILL.md 파일에서 설명 읽기 (frontmatter YAML 파싱)
			description := ""
			skillMdPath := filepath.Join(realPath, "SKILL.md")
			if content, err := ioutil.ReadFile(skillMdPath); err == nil {
				text := string(content)
				// frontmatter 파싱: --- 사이의 YAML에서 description 추출
				if strings.HasPrefix(text, "---") {
					parts := strings.SplitN(text, "---", 3)
					if len(parts) >= 2 {
						frontmatter := parts[1]
						for _, line := range strings.Split(frontmatter, "\n") {
							line = strings.TrimSpace(line)
							if strings.HasPrefix(line, "description:") {
								description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
								break
							}
						}
					}
				}
				// frontmatter에서 못 찾으면 # 제목 또는 첫 번째 줄 사용
				if description == "" {
					lines := strings.Split(text, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "---" {
							continue
						}
						if line != "" && !strings.HasPrefix(line, "#") && !strings.Contains(line, ":") {
							description = line
							break
						} else if strings.HasPrefix(line, "# ") {
							description = strings.TrimPrefix(line, "# ")
							break
						}
					}
				}
				// 설명이 너무 길면 자르기
				runes := []rune(description)
				if len(runes) > 80 {
					description = string(runes[:80]) + "..."
				}
			} else {
				log.Printf("SKILL.md 읽기 실패: %v", err)
			}

			skills = append(skills, SkillInfo{
				Name:        entry.Name(),
				Path:        realPath,
				Description: description,
			})
			log.Printf("스킬 추가됨: %s", entry.Name())
		}

		log.Printf("총 스킬 수: %d", len(skills))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": skills,
		})
	})

	// 이미지 업로드 API
	http.HandleFunc("/api/upload", func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		log.Printf("[업로드] 요청 시작")

		enableCORS(w)
		if r.Method == "OPTIONS" {
			log.Printf("[업로드] OPTIONS preflight 응답")
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		// 세션 ID 확인
		sessionID := r.URL.Query().Get("sessionId")
		if sessionID == "" {
			http.Error(w, "sessionId required", http.StatusBadRequest)
			return
		}
		log.Printf("[업로드] 세션: %s (%.0fms)", sessionID, time.Since(startTime).Seconds()*1000)

		// 멀티파트 파싱 (최대 10MB)
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			log.Printf("[업로드] 멀티파트 파싱 실패: %v", err)
			http.Error(w, "파일 파싱 실패: "+err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("[업로드] 멀티파트 파싱 완료 (%.0fms)", time.Since(startTime).Seconds()*1000)

		file, handler, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "이미지 파일 필요: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		log.Printf("[업로드] 파일: %s, 크기: %d bytes (%.0fms)", handler.Filename, handler.Size, time.Since(startTime).Seconds()*1000)

		// 허용된 이미지 타입 확인
		contentType := handler.Header.Get("Content-Type")
		allowedTypes := map[string]string{
			"image/png":  ".png",
			"image/jpeg": ".jpg",
			"image/gif":  ".gif",
			"image/webp": ".webp",
		}
		ext, ok := allowedTypes[contentType]
		if !ok {
			http.Error(w, "허용되지 않은 이미지 타입: "+contentType, http.StatusBadRequest)
			return
		}

		// temp/세션ID 디렉토리 생성
		tempDir := filepath.Join("temp", sessionID)
		if err := os.MkdirAll(tempDir, 0755); err != nil {
			http.Error(w, "디렉토리 생성 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 파일명 생성 (타임스탬프)
		filename := time.Now().Format("20060102_150405") + ext
		filePath := filepath.Join(tempDir, filename)

		// 파일 저장
		dst, err := os.Create(filePath)
		if err != nil {
			http.Error(w, "파일 생성 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		written, err := dst.ReadFrom(file)
		if err != nil {
			http.Error(w, "파일 저장 실패: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("[업로드] 파일 저장 완료: %d bytes (%.0fms)", written, time.Since(startTime).Seconds()*1000)

		// 절대 경로 반환
		absPath, _ := filepath.Abs(filePath)
		log.Printf("[업로드] 완료: %s (총 %.0fms)", absPath, time.Since(startTime).Seconds()*1000)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"path":     absPath,
			"filename": filename,
		})
	})

	// 이미지 파일 서빙 API (절대 경로 지원)
	http.HandleFunc("/api/image", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 쿼리 파라미터로 절대 경로 받기
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}

		// 파일 존재 확인
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			log.Printf("이미지 파일 없음: %s", filePath)
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}

		// Content-Type 설정
		ext := strings.ToLower(filepath.Ext(filePath))
		contentTypes := map[string]string{
			".png":  "image/png",
			".jpg":  "image/jpeg",
			".jpeg": "image/jpeg",
			".gif":  "image/gif",
			".webp": "image/webp",
			".bmp":  "image/bmp",
		}
		if ct, ok := contentTypes[ext]; ok {
			w.Header().Set("Content-Type", ct)
		}

		log.Printf("이미지 서빙: %s", filePath)
		http.ServeFile(w, r, filePath)
	})

	// Persona API
	personasDir := filepath.Join(ricoBasePath, "context", "personas")
	activePersonaPath := filepath.Join(personasDir, "active.json")

	// 활성 페르소나 이름 가져오기
	getActivePersona := func() string {
		data, err := ioutil.ReadFile(activePersonaPath)
		if err != nil {
			return "default"
		}
		var active struct {
			Current string `json:"current"`
		}
		if json.Unmarshal(data, &active) != nil || active.Current == "" {
			return "default"
		}
		return active.Current
	}

	// 페르소나 config.json 가져오기
	http.HandleFunc("/api/persona", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		activePersona := getActivePersona()
		configPath := filepath.Join(personasDir, activePersona, "config.json")

		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			// 기본값 반환
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"name":        "Assistant",
				"avatar":      "A",
				"accentColor": "#4fd1c5",
				"typingMessages": map[string][]string{
					"default": {"Thinking...", "Processing...", "Working on it..."},
					"ko":      {"생각하는 중...", "처리 중...", "작업 중..."},
					"en":      {"Thinking...", "Processing...", "Working on it..."},
				},
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	// 활성 페르소나 변경
	http.HandleFunc("/api/persona/active", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"current": getActivePersona()})
			return
		}

		if r.Method == "POST" {
			var req struct {
				Persona string `json:"persona"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "요청 파싱 실패", http.StatusBadRequest)
				return
			}

			// 페르소나 폴더 존재 확인
			personaPath := filepath.Join(personasDir, req.Persona)
			if _, err := os.Stat(personaPath); os.IsNotExist(err) {
				http.Error(w, "페르소나 없음", http.StatusNotFound)
				return
			}

			// active.json 업데이트
			activeData := map[string]string{"current": req.Persona}
			data, _ := json.MarshalIndent(activeData, "", "  ")
			if err := ioutil.WriteFile(activePersonaPath, data, 0644); err != nil {
				http.Error(w, "저장 실패", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// 페르소나 목록
	http.HandleFunc("/api/personas", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		entries, err := os.ReadDir(personasDir)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"personas": []string{}})
			return
		}

		type PersonaInfo struct {
			Name        string `json:"name"`
			FolderName  string `json:"folderName"`
			Avatar      string `json:"avatar"`
			AccentColor string `json:"accentColor"`
		}

		personas := make([]PersonaInfo, 0)
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "active.json" {
				continue
			}

			configPath := filepath.Join(personasDir, entry.Name(), "config.json")
			configData, err := ioutil.ReadFile(configPath)
			if err != nil {
				continue
			}

			var config struct {
				Name        string `json:"name"`
				Avatar      string `json:"avatar"`
				AccentColor string `json:"accentColor"`
			}
			if json.Unmarshal(configData, &config) != nil {
				continue
			}

			personas = append(personas, PersonaInfo{
				Name:        config.Name,
				FolderName:  entry.Name(),
				Avatar:      config.Avatar,
				AccentColor: config.AccentColor,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"personas": personas,
			"active":   getActivePersona(),
		})
	})

	// SOUL.md API (새 personas 구조 사용 - 동적 경로)
	getSoulPath := func() string {
		return filepath.Join(personasDir, getActivePersona(), "SOUL.md")
	}

	// SOUL 읽기 (동적 경로)
	http.HandleFunc("/api/soul", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		currentSoulPath := getSoulPath()

		if r.Method == "GET" {
			content, err := ioutil.ReadFile(currentSoulPath)
			if err != nil {
				http.Error(w, "SOUL.md 읽기 실패", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"content": string(content)})
			return
		}

		if r.Method == "POST" {
			var req struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "요청 파싱 실패", http.StatusBadRequest)
				return
			}
			if err := ioutil.WriteFile(currentSoulPath, []byte(req.Content), 0644); err != nil {
				http.Error(w, "SOUL.md 저장 실패", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// SOUL 생성 (캐릭터 검색 또는 직접 입력)
	http.HandleFunc("/api/soul/generate", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("SOUL 생성 API 호출됨")

		var req struct {
			CharacterName string `json:"characterName"` // 캐릭터 검색용
			Name          string `json:"name"`          // 직접 입력
			Personality   string `json:"personality"`
			Gender        string `json:"gender"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("SOUL 생성 요청 파싱 실패: %v", err)
			http.Error(w, "요청 파싱 실패", http.StatusBadRequest)
			return
		}
		log.Printf("SOUL 생성 요청: characterName=%s, name=%s", req.CharacterName, req.Name)

		// SOUL 생성 규칙 파일 경로
		soulGeneratorPath := filepath.Join(ricoBasePath, "context", "SOUL_GENERATOR.md")

		var prompt string
		if req.CharacterName != "" {
			// 캐릭터 검색 모드 - 간단한 프롬프트
			prompt = fmt.Sprintf(`"%s" 캐릭터의 SOUL 문서를 만들어줘.`, req.CharacterName)
		} else {
			// 직접 입력 모드
			prompt = fmt.Sprintf(`다음 정보로 SOUL 문서를 만들어줘:
- 이름: %s
- 성격: %s
- 성별: %s`, req.Name, req.Personality, req.Gender)
		}

		// 캐릭터 이름 결정 (파일명용)
		characterName := req.CharacterName
		if characterName == "" {
			characterName = req.Name
		}

		// 비동기로 생성 시작
		soulGenerateMu.Lock()
		soulGenerateResult = SoulGenerateStatus{Status: "generating"}
		soulGenerateMu.Unlock()

		go func(charName string, userPrompt string) {
			log.Printf("SOUL 생성 시작: charName=%s, prompt=%s", charName, userPrompt)

			// stdin으로 프롬프트 전달
			cmd := exec.Command("claude", "--print", "--model", "opus", "--append-system-prompt-file", soulGeneratorPath, "--dangerously-skip-permissions", "-p", "-")
			cmd.Dir = ricoBasePath
			cmd.Env = getCleanEnvForClaude()

			stdin, err := cmd.StdinPipe()
			if err != nil {
				log.Printf("SOUL 생성 stdin 파이프 실패: %v", err)
				soulGenerateMu.Lock()
				soulGenerateResult = SoulGenerateStatus{Status: "error", Error: "stdin 파이프 실패"}
				soulGenerateMu.Unlock()
				return
			}

			// 비동기로 stdin에 프롬프트 쓰기
			go func() {
				defer stdin.Close()
				stdin.Write([]byte(userPrompt))
			}()

			output, err := cmd.CombinedOutput()
			log.Printf("SOUL 생성 Claude 응답: err=%v, output_len=%d", err, len(output))

			soulGenerateMu.Lock()
			if err != nil {
				log.Printf("SOUL 생성 Claude 호출 실패: %v, 출력: %s", err, string(output))
				soulGenerateResult = SoulGenerateStatus{Status: "error", Error: err.Error() + ": " + string(output)}
			} else {
				content := cleanSoulContent(string(output))
				log.Printf("SOUL 생성 완료: %d bytes, content preview: %s", len(content), content[:min(100, len(content))])

				// souls 폴더에 저장 (타임스탬프 기반 파일명)
				soulsDir := filepath.Join(ricoBasePath, "context", "souls")
				os.MkdirAll(soulsDir, 0755)
				soulFileName := fmt.Sprintf("soul.%d.md", time.Now().UnixMilli())
				soulFilePath := filepath.Join(soulsDir, soulFileName)
				if err := ioutil.WriteFile(soulFilePath, []byte(content), 0644); err != nil {
					log.Printf("SOUL 파일 저장 실패: %v", err)
				} else {
					log.Printf("SOUL 파일 저장됨: %s", soulFilePath)
				}

				soulGenerateResult = SoulGenerateStatus{Status: "done", Content: content}
				sendPushNotification("SOUL 생성 완료", charName+" SOUL이 준비되었습니다!")
			}
			soulGenerateMu.Unlock()
		}(characterName, prompt)

		// 즉시 응답 (생성 시작됨)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "generating"})
	})

	// SOUL 생성 상태 확인 API
	http.HandleFunc("/api/soul/generate/status", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		soulGenerateMu.RLock()
		result := soulGenerateResult
		soulGenerateMu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})

	// SOUL 생성 상태 초기화 API
	http.HandleFunc("/api/soul/generate/reset", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		soulGenerateMu.Lock()
		soulGenerateResult = SoulGenerateStatus{}
		soulGenerateMu.Unlock()

		log.Printf("SOUL 생성 상태 초기화됨")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "reset"})
	})

	// SOUL 수정 요청 API
	http.HandleFunc("/api/soul/modify", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		log.Printf("SOUL 수정 API 호출됨")

		var req struct {
			CurrentSoul string `json:"currentSoul"`
			Request     string `json:"request"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("SOUL 수정 요청 파싱 실패: %v", err)
			http.Error(w, "요청 파싱 실패", http.StatusBadRequest)
			return
		}
		log.Printf("SOUL 수정 요청: %s", req.Request)

		prompt := fmt.Sprintf(`아래 SOUL 설정을 유저의 요청에 맞게 수정해줘.

현재 SOUL:
%s

수정 요청:
%s

설명 없이 "# SOUL"부터 바로 시작해서 출력해줘.`, req.CurrentSoul, req.Request)

		// 비동기로 수정 시작
		soulGenerateMu.Lock()
		soulGenerateResult = SoulGenerateStatus{Status: "generating"}
		soulGenerateMu.Unlock()

		go func(userPrompt string) {
			log.Printf("SOUL 수정 Claude 호출 시작 (비동기, Opus)")
			cmd := exec.Command("claude", "--print", "--model", "opus", "--dangerously-skip-permissions", "-p", "-")
			cmd.Dir = ricoBasePath
			cmd.Env = getCleanEnvForClaude() // CLAUDECODE 환경 변수 제거

			stdin, err := cmd.StdinPipe()
			if err != nil {
				log.Printf("SOUL 수정 stdin 파이프 실패: %v", err)
				soulGenerateMu.Lock()
				soulGenerateResult = SoulGenerateStatus{Status: "error", Error: "stdin 파이프 실패"}
				soulGenerateMu.Unlock()
				return
			}

			go func() {
				defer stdin.Close()
				stdin.Write([]byte(userPrompt))
			}()

			output, err := cmd.CombinedOutput()

			soulGenerateMu.Lock()
			if err != nil {
				log.Printf("SOUL 수정 Claude 호출 실패: %v, 출력: %s", err, string(output))
				soulGenerateResult = SoulGenerateStatus{Status: "error", Error: err.Error()}
			} else {
				log.Printf("SOUL 수정 완료: %d bytes", len(output))
				soulGenerateResult = SoulGenerateStatus{Status: "done", Content: cleanSoulContent(string(output))}
			}
			soulGenerateMu.Unlock()
		}(prompt)

		// 즉시 응답 (수정 시작됨)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "generating"})
	})

	// 설정 API
	http.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "GET" {
			// 현재 설정 조회
			settingsMu.RLock()
			data, _ := json.Marshal(ricoSettings)
			settingsMu.RUnlock()

			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}

		if r.Method == "POST" {
			// 설정 변경
			var newSettings RicoSettings
			if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
				http.Error(w, "설정 파싱 실패: "+err.Error(), http.StatusBadRequest)
				return
			}

			settingsMu.Lock()
			ricoSettings = newSettings
			settingsMu.Unlock()

			saveSettings()

			log.Printf("설정 변경됨")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// 푸시 구독 등록 API
	http.HandleFunc("/api/push/subscribe", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}

		var sub webpush.Subscription
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			http.Error(w, "구독 정보 파싱 실패: "+err.Error(), http.StatusBadRequest)
			return
		}

		// 구독 저장 (endpoint를 키로)
		pushMu.Lock()
		pushSubscriptions[sub.Endpoint] = &sub
		pushMu.Unlock()

		// 파일에 저장
		savePushSubscriptions()

		log.Printf("푸시 구독 등록됨: %s", sub.Endpoint[:50])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	})

	addr := ":" + SERVER_PORT

	// SSL 인증서가 설정되어 있으면 HTTPS, 아니면 HTTP
	if SSL_CERT_FILE != "" && SSL_KEY_FILE != "" {
		log.Printf("Rico 브릿지 서버 시작 (HTTPS): %s", addr)
		log.Fatal(http.ListenAndServeTLS(addr, SSL_CERT_FILE, SSL_KEY_FILE, nil))
	} else {
		log.Printf("Rico 브릿지 서버 시작 (HTTP): %s", addr)
		log.Printf("  주의: SSL 인증서가 설정되지 않아 HTTP로 실행됩니다.")
		log.Fatal(http.ListenAndServe(addr, nil))
	}
}
