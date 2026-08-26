// Package httpapi 提供 /api 前缀的 HTTP 接口层，职责单一：解析请求、调用 service、返回 JSON。
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"task260-adnattr/internal/model"
	"task260-adnattr/internal/service"
)

// Server HTTP 服务。
type Server struct {
	svc *service.Service
	mux *http.ServeMux
}

// New 构造 Server 并注册全部路由。
func New(svc *service.Service) *Server {
	s := &Server{svc: svc, mux: http.NewServeMux()}
	s.routes()
	return s
}

// Handler 返回可挂载的 http.Handler。
func (s *Server) Handler() http.Handler { return s.mux }

// routes 注册全部 /api 路由（共 29 个端点）。
func (s *Server) routes() {
	// 文库批次
	s.mux.HandleFunc("POST /api/libraries", s.handleCreateLibrary)
	s.mux.HandleFunc("GET /api/libraries", s.handleListLibraries)
	s.mux.HandleFunc("GET /api/libraries/{id}", s.handleGetLibrary)
	s.mux.HandleFunc("POST /api/libraries/{id}/advance", s.handleAdvanceLibrary)
	s.mux.HandleFunc("POST /api/libraries/{id}/seal", s.handleSealLibrary)
	s.mux.HandleFunc("POST /api/libraries/{id}/analyze", s.handleAnalyzeLibrary)
	s.mux.HandleFunc("POST /api/libraries/{id}/cluster", s.handleClusterLibrary)
	s.mux.HandleFunc("POST /api/libraries/{id}/exclude-batch", s.handleExcludeBatch)
	// 片段与片段簇
	s.mux.HandleFunc("POST /api/fragments", s.handleIngestFragment)
	s.mux.HandleFunc("POST /api/fragments/batch", s.handleBatchIngestFragments)
	s.mux.HandleFunc("GET /api/fragments", s.handleListFragments)
	s.mux.HandleFunc("GET /api/fragments/{id}", s.handleGetFragment)
	s.mux.HandleFunc("GET /api/fragment-clusters", s.handleListClusters)
	s.mux.HandleFunc("GET /api/fragment-clusters/{id}", s.handleGetCluster)
	s.mux.HandleFunc("POST /api/fragment-clusters/{id}/classify", s.handleClassifyCluster)
	// 对照
	s.mux.HandleFunc("POST /api/controls", s.handleCreateControl)
	s.mux.HandleFunc("GET /api/controls", s.handleListControls)
	s.mux.HandleFunc("GET /api/controls/{id}", s.handleGetControl)
	s.mux.HandleFunc("POST /api/controls/{id}/associate", s.handleAssociateControl)
	// 损伤与归因
	s.mux.HandleFunc("GET /api/damage-profiles", s.handleGetDamageProfile)
	s.mux.HandleFunc("GET /api/attributions", s.handleListAttributions)
	s.mux.HandleFunc("GET /api/attributions/{id}", s.handleGetAttribution)
	s.mux.HandleFunc("POST /api/attributions/{id}/confirm", s.handleConfirmAttribution)
	// 可信度快照
	s.mux.HandleFunc("POST /api/snapshots", s.handlePublishSnapshot)
	s.mux.HandleFunc("GET /api/snapshots", s.handleListSnapshots)
	s.mux.HandleFunc("GET /api/snapshots/{id}", s.handleGetSnapshot)
	s.mux.HandleFunc("POST /api/snapshots/{id}/supersede", s.handleSupersedeSnapshot)
	// 统计与自检
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/self-check", s.handleSelfCheck)
}

// pathID 解析路径参数 {id} 为 int64。
func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// decodeJSON 带大小限制（1 MiB）与未知字段拒绝地解码请求体。
// 请求体必须恰好包含一个合法 JSON 对象；首个对象之后若仍有任何内容
// （包括第二个 JSON 对象或非 JSON 垃圾），一律视为参数错误，避免只
// 解析第一个对象便误返回成功、产生半截业务写入。
func decodeJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) >= 1<<20 {
		return errors.New("httpapi: request body too large")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	// 首个对象解码成功后，请求体必须已耗尽（仅允许尾随空白）。
	// 若仍能解码出第二个值，或剩余内容不是合法 JSON，Decode 会返回
	// 非 io.EOF 错误，此时整体视为参数错误。
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("httpapi: request body must contain exactly one JSON object")
	}
	return nil
}

// writeJSON 输出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeErr 输出错误响应。
func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// statusFromErr 将领域错误映射为 HTTP 状态码。
func statusFromErr(err error) (int, string) {
	switch {
	case errors.Is(err, model.ErrUnknownLibrary),
		errors.Is(err, model.ErrUnknownCluster),
		errors.Is(err, model.ErrUnknownControl):
		return http.StatusNotFound, err.Error()
	case errors.Is(err, model.ErrInvalidStatus),
		errors.Is(err, model.ErrSealed),
		errors.Is(err, model.ErrInvalidBase),
		errors.Is(err, model.ErrSelfReference),
		errors.Is(err, model.ErrBatchCycle),
		errors.Is(err, model.ErrControlMissing),
		errors.Is(err, model.ErrEmptyName),
		errors.Is(err, model.ErrNonPositive):
		return http.StatusBadRequest, err.Error()
	default:
		return http.StatusInternalServerError, err.Error()
	}
}
