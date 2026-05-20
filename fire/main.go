package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type tableConfig struct {
	TableHeader []string        `json:"table_header"`
	TableData   [][]interface{} `json:"table_data"`
}

const dataFile = "fire.json"

func main() {
	http.HandleFunc("/api/data", dataHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/fire.html", http.StatusFound)
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	log.Println("server started at http://localhost:8001/fire.html")
	log.Fatal(http.ListenAndServe(":8001", nil))
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
	data, err := os.ReadFile(dataFile)
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

	if err := atomicWrite(dataFile, data, 0644); err != nil {
		http.Error(w, "failed to save data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".fire-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	cleanup = false
	return syncDir(dir)
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()
	if err := f.Sync(); err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}
