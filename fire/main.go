package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type tableConfig struct {
	TableHeader []string        `json:"table_header"`
	TableData   [][]interface{} `json:"table_data"`
}

const dataFile = "fire.json"

var cache = struct {
	sync.Mutex
	data  []byte
	dirty bool
}{}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	go flushCacheLoop()

	http.HandleFunc("/api/data", dataHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/fire.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	srv := &http.Server{Addr: ":8001"}
	go func() {
		<-stop
		if err := flushCache(); err != nil {
			log.Printf("flush cache on exit failed: %v", err)
		}
		_ = srv.Close()
	}()

	log.Println("server started at http://localhost:8001/fire.html")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	if err := flushCache(); err != nil {
		log.Printf("final flush failed: %v", err)
	}
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		readData(w)
	case http.MethodPost:
		writeData(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func readData(w http.ResponseWriter) {
	data, err := getCachedData()
	if err != nil {
		http.Error(w, "failed to read data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(data)
}

func writeData(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var cfg tableConfig
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&cfg); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if len(cfg.TableHeader) == 0 || cfg.TableData == nil {
		http.Error(w, "invalid table config", http.StatusBadRequest)
		return
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		http.Error(w, "failed to encode data", http.StatusInternalServerError)
		return
	}
	data = append(data, '\n')

	setCachedData(data)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func flushCacheLoop() {
	for {
		time.Sleep(time.Minute)
		if err := flushCache(); err != nil {
			log.Printf("flush cache failed: %v", err)
		}
	}
}

func getCachedData() ([]byte, error) {
	cache.Lock()
	defer cache.Unlock()

	if cache.data != nil {
		return append([]byte(nil), cache.data...), nil
	}

	data, err := os.ReadFile(dataFile)
	if err != nil {
		return nil, err
	}
	cache.data = append([]byte(nil), data...)
	return append([]byte(nil), data...), nil
}

func setCachedData(data []byte) {
	cache.Lock()
	defer cache.Unlock()

	cache.data = append([]byte(nil), data...)
	cache.dirty = true
}

func flushCache() error {
	cache.Lock()
	defer cache.Unlock()

	if !cache.dirty || cache.data == nil {
		return nil
	}
	if err := saveData(dataFile, cache.data, 0644); err != nil {
		return err
	}
	cache.dirty = false
	return nil
}

func saveData(path string, data []byte, perm os.FileMode) error {
	err := os.WriteFile(path, data, perm)
	if err != nil {
		log.Printf("failed to write data to %s: %v", path, err)
	} else {
		log.Printf("saved data to %s success", path)
	}
	return err
}
