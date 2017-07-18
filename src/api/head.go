package api

import (
	"log"
	"net/http"
	"time"
)

// получаем key, проверяем, находиться ли ключ в db
func (s *api) Head(rw http.ResponseWriter, req *http.Request) {
	relationPath := req.URL.Path
	if req.URL.RawQuery != "" {
		relationPath = relationPath + "?" + req.URL.RawQuery
	}
	beginAt := time.Now()

	stmt, err := s.getDB().Prepare(`select exist($1)`)
	if err != nil {
		log.Printf("[ERROR] HEAD prepare: %s\n", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_PREPARE}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer stmt.Close()
	rows, err := stmt.Query(relationPath)
	if err != nil {
		log.Printf("[ERROR] HEAD exec: %s\n", err.Error())
		rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_EXEC}
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var exists bool
		err := rows.Scan(&exists)
		if err != nil {
			log.Printf("[ERROR] HEAD parse: %s\n", err.Error())
			rw.Header()[ERROR_HEADER] = []string{ERROR_CODE_EXEC}
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		if exists {
			rw.WriteHeader(http.StatusOK)
		} else {
			rw.WriteHeader(http.StatusNotFound)
		}
		log.Printf("[INFO] HEAD %s %s completed\n", relationPath, time.Now().Sub(beginAt))
		return
	}

}
