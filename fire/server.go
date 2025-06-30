package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// FireData 表示保存的数据结构
type FireData struct {
	Timestamp string      `json:"timestamp"`
	TableData interface{} `json:"tableData"`
	ColOrder  []string    `json:"colOrder"`
}

// Response 表示API响应结构
type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// FireDataHandler 处理fire.json相关的请求
type FireDataHandler struct{}

// ServeHTTP 实现http.Handler接口
func (h *FireDataHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 只处理/fire.json路径的请求
	if r.URL.Path != "/fire.json" {
		http.NotFound(w, r)
		return
	}

	if r.Method == "POST" {
		h.handleSaveData(w, r)
	} else if r.Method == "GET" {
		h.handleLoadData(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSaveData 处理数据保存请求
func (h *FireDataHandler) handleSaveData(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("读取请求体失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Status:  "error",
			Message: "读取请求体失败",
		})
		return
	}
	defer r.Body.Close()

	// 解析JSON数据
	var fireData FireData
	if err := json.Unmarshal(body, &fireData); err != nil {
		log.Printf("解析JSON失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Status:  "error",
			Message: "解析JSON失败",
		})
		return
	}

	// 设置时间戳
	fireData.Timestamp = time.Now().Format(time.RFC3339)

	// 保存到文件
	if err := h.saveToFile(fireData); err != nil {
		log.Printf("保存文件失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Status:  "error",
			Message: "保存文件失败",
		})
		return
	}

	// 返回成功响应
	response := Response{
		Status:  "success",
		Message: "数据已保存",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("编码响应失败: %v", err)
	}
}

// handleLoadData 处理数据加载请求
func (h *FireDataHandler) handleLoadData(w http.ResponseWriter, r *http.Request) {
	// 检查文件是否存在
	if _, err := os.Stat("fire.json"); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(Response{
			Status:  "error",
			Message: "文件不存在",
		})
		return
	}

	// 读取文件
	data, err := os.ReadFile("fire.json")
	if err != nil {
		log.Printf("读取文件失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{
			Status:  "error",
			Message: "读取文件失败",
		})
		return
	}

	// 返回文件内容
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// saveToFile 保存数据到文件
func (h *FireDataHandler) saveToFile(data FireData) error {
	// 将数据转换为JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化JSON失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile("fire.json", jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	log.Printf("数据已保存到 fire.json")
	return nil
}

// setCORSHeaders 设置CORS头
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

// 静态文件处理器
type StaticFileHandler struct {
	fileServer http.Handler
}

func NewStaticFileHandler() *StaticFileHandler {
	return &StaticFileHandler{
		fileServer: http.FileServer(http.Dir(".")),
	}
}

func (h *StaticFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 使用默认文件服务器处理静态文件
	h.fileServer.ServeHTTP(w, r)
}

// CORS中间件
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS头
		setCORSHeaders(w)
		
		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		// 调用下一个处理器
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := 8000
	
	// 创建路由
	mux := http.NewServeMux()
	
	// 注册处理器
	fireHandler := &FireDataHandler{}
	staticHandler := NewStaticFileHandler()
	
	mux.Handle("/fire.json", fireHandler)
	mux.Handle("/", staticHandler)

	// 创建服务器，使用CORS中间件
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	fmt.Printf("服务器启动在 http://localhost:%d\n", port)
	fmt.Println("按 Ctrl+C 停止服务器")
	
	// 启动服务器
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
} 