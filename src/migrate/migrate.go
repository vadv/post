package migrate

import (
	"database/sql"

	_ "github.com/lib/pq"
)

func Run(connString string) error {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return err
	}
	_, err = db.Exec(v1)
	return err
}
