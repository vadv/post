# Схема

Приложение состоит из pg, api и nginx.
(pg <-> api <-> nginx) <-> client

api умеет сохранять и выгружать данные в pg, nginx работает в режиме proxy_store для минимизации общения с pg.

# Деплой

```
$ sudo yum install postgresql96-server postgresql96-contrib postgresql96-plpython
$ psql -c 'create database data'
$ post -run-migrate -connection-string "postgresql://127.0.0.1/data"
$ sudo yum install nginx -y
$ post -workdir /var/tmp/tmpfs -connection-string "postgresql://127.0.0.1/data?user=data"
```

# ENTERPOINT для конечного пользователя

Пример использования:
```

$ curl -X POST 127.0.0.1/images/ololo/file.jpg --data-binary @/tmp/big_file_2 -s -o /dev/null -w "%{http_code}"
201

на повторный реквест nginx ответит 405
$ curl -X POST 127.0.0.1/images/ololo/file.jpg --data-binary @/tmp/big_file_2 -s -o /dev/null -w "%{http_code}"
405

ответ идет от апстрима, пишем ответ в proxy_cache
$ curl -s 127.0.0.1/images/ololo/file.jpg -s -o /dev/null -w "%{http_code}"
200

ответ идет от nginx из proxy_cache
$ curl -s 127.0.0.1/images/ololo/file.jpg -s -o /dev/null -w "%{http_code}"
200

на неизвестный файл будет ответ 404
$curl -s 127.0.0.1/images/ololo/file1.jpg -s -o /dev/null -w "%{http_code}"
404

nginx игнорирует параметры
$ curl -s "127.0.0.1/images/ololo/file.jpg?version=12" -s -o /dev/null -w "%{http_code}"
200
```

при этом в базе оно записалось 1 раз и закэшировалось nginx'ом после первого реквеста
```
2017/07/18 03:45:19 [INFO] POST /images/ololo/file.jpg 0.570350505s completed
```

Примерный конфиг nginx:

```
upstream post_upstream {
  server 127.0.0.1:8080;
  keepalive 256;
}

server {

  listen 80 default_server;

  client_max_body_size 100m;
  root /var/www;

  location /images/ {
    try_files $uri @fetch;
  }

  location @fetch {
    proxy_pass http://post_upstream;
    proxy_store on;
    proxy_store_access user:rw group:rw all:r;
    root /var/www/;
  }

}
```

# API

  * создать файл:  `POST /relative/path/to/file/name`
  * получить файл: `GET  /relative/path/to/file/name`

удаления нет - так как это запрещенная операция по выбиванию ключа.

# SQL

Со стороны SQL была задача предоставить простой интерфейс к стораджу:
  * сохраняем в базу: `select IMPORT('key', '/path/to/exists_file')`
  * проверяем, есть ли в базе указаный ключ: `select EXIST('key')`
  * выгружаем из базы по ключу в файл: `select EXPORT('key', '/path/to/new_file')`
  * удаляем ключ: `select DELETE('key')`
  * проводим дедубликацию `select DEDUPLICATE()`

В кишочках:
  * хранение происходит с дедубликацией(производится по требованию).
  * данные хранятся во многих партициях, партиции бьются по md5 от key для того чтобы достичь честности распределения.
  * запросы в указанные функции идут четко в относледованую таблицу, минуя родительскую.
  * на центральную родительскую таблицу запрещен insert.
  * на каждой относледованной таблицы весит check.

