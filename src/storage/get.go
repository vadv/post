package storage

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// получаем key, выгружаем из db в workdir,
// открываем файл, читаем его в память, отдаем клиенту
func (s *storage) Get(rw http.ResponseWriter, req *http.Request) {

	relationPath := req.URL.Path
	if req.URL.RawQuery != "" {
		relationPath = relationPath + "?" + req.URL.RawQuery
	}
	tmpPath := filepath.Join(s.workDir, url.QueryEscape(relationPath))
	beginAt := time.Now()

	stmt, err := s.getDB().Prepare(`select EXPORT($1, $2)`)
	if err != nil {
		log.Printf("[ERROR] GET prepare: %s\n", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_PREPARE}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(relationPath, tmpPath)
	if err != nil {
		if strings.Contains(err.Error(), " not found") {
			log.Printf("[ERROR] GET %s: not found\n", relationPath)
			rw.WriteHeader(http.StatusNotFound)
		} else {
			log.Printf("[ERROR] GET exec: %s\n", err.Error())
			rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_EXEC}
			rw.WriteHeader(http.StatusInternalServerError)
		}
		return
	}
	// файл был записан базой на диск, читаем его в память полностью и отдаем.
	// на самом деле у responseWriter'a я не наншел метод Read,
	// а с другой стороны было бы хорошо проверить, перекачался ли файл.
	data, err := ioutil.ReadFile(tmpPath)
	if err != nil {
		log.Printf("[ERROR] GET exec: %s\n", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_OPEN}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpPath)

	rw.Write(data)
	log.Printf("[INFO] GET %s %s completed\n", relationPath, time.Now().Sub(beginAt))
}
