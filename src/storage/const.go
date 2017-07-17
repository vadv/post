package storage

const (
	UPLOADED_FILE_PREFIX = `.tmp`
	DB_MAX_OPEN          = 10
	DB_MAX_IDLE          = 10
)

// сообщаем в хидерах ошибку
const (
	ERROR_HEADER       = `Error`
	ERROR_CODE_PREPARE = `OPEN DB`
	ERROR_CODE_EXEC    = `EXEC DB`
	ERROR_CODE_READ    = `READ FILE`
	ERROR_CODE_WRITE   = `WRITE FILE`
	ERROR_CODE_OPEN    = `OPEN FILE`
)
