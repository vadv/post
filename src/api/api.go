package api

import (
	"database/sql"
	"log"
	"net/http"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

type api struct {
	sync.Mutex
	workDir    string
	connString string
	db         *sql.DB
}

func openDb(connString string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(DB_MAX_OPEN)
	db.SetMaxIdleConns(DB_MAX_IDLE)
	log.Printf("[INFO] open db `%s`: successfully\n", connString)
	return db, nil
}

func New(workDir, connString string) (*api, error) {
	result := &api{workDir: workDir, connString: connString}
	db, err := openDb(result.connString)
	if err != nil {
		return nil, err
	}
	result.db = db
	go result.dbPinger()
	return result, nil
}

func (s *api) ServeHTTP(rw http.ResponseWriter, req *http.Request) {

	// ставим здесь запрет на params, так как мы хотим использовать nginx proxy_store
	// если мы хотим использовать версии, то в дальнейшем можно снять запрет но перейти на nginx proxy_cache
	if req.URL.RawQuery != "" {
		log.Printf("[ERROR] decline %s: params not empty\n", req.URL.String())
		rw.WriteHeader(http.StatusNotAcceptable)
		return
	}

	switch {
	case req.Method == "GET":
		s.Get(rw, req)
	case req.Method == "POST":
		s.Post(rw, req)
	default:
		log.Printf("[ERROR] unknown verb: %s\n", req.Method)
		rw.WriteHeader(http.StatusNotAcceptable)
	}
}

func (s *api) getDB() *sql.DB {
	s.Lock()
	defer s.Unlock()
	return s.db
}

func (s *api) dbPinger() {
	ticker := time.NewTicker(time.Minute)
	for {
		select {
		case <-ticker.C:
			s.Lock()
			if err := s.db.Ping(); err != nil {
				log.Printf("[ERROR] db ping: %s\n", err.Error())
				if db, err := openDb(s.connString); err != nil {
					log.Printf("[ERROR] reconnect: %s\n", err.Error())
				} else {
					log.Printf("[INFO] reconnect successfully\n")
					s.db = db
				}
			}
			s.Unlock()
		}
	}
}
