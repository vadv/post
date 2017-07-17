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

// читаем содержимое body, пишем его в файл, находящийся в workdir, загружаем его в db
func (s *storage) Post(rw http.ResponseWriter, req *http.Request) {

	relationPath := req.URL.Path
	if req.URL.RawQuery != "" {
		relationPath = relationPath + "?" + req.URL.RawQuery
	}
	tmpPath := filepath.Join(s.workDir, url.QueryEscape(relationPath))
	beginAt := time.Now()

	// скачиваем файл во временный файл
	os.MkdirAll(filepath.Dir(tmpPath), 0755)
	fd, err := os.Create(tmpPath)
	if err != nil {
		log.Printf("[ERROR] POST create file: %s", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_OPEN}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer fd.Close()
	defer os.Remove(tmpPath)

	// читаем тело в память, так как им же нам для nginx нужно ответить 200 чтобы он сразу закэшировал
	// можно ответить 201, тогда он кэшировать не будет
	content, errRead := ioutil.ReadAll(req.Body)
	if errRead != nil || ioutil.WriteFile(tmpPath, content, 0777) != nil {
		log.Printf("[ERROR] POST write file: %s", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_WRITE}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// записываем в базу
	stmt, err := s.getDB().Prepare(`select IMPORT($1, $2)`)
	if err != nil {
		log.Printf("[ERROR] POST prepare: %s\n", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_PREPARE}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(relationPath, tmpPath)
	if err != nil {
		if strings.Contains(err.Error(), " already exists") {
			log.Printf("[ERROR] POST %s: already extist\n", relationPath)
			rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_EXEC}
			rw.WriteHeader(http.StatusConflict)
		} else {
			log.Printf("[ERROR] POST exec: %s\n", err.Error())
			rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_EXEC}
			rw.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// nginx proxy_store в ответ на 200 закэширует ответ
	rw.Write(content)
	//rw.WriteHeader(http.StatusCreated)
	log.Printf("[INFO] POST %s %s completed\n", relationPath, time.Now().Sub(beginAt))
}
