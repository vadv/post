create or replace language plpythonu;
do $$
  plpy.execute("""
create table storage(
  key         text not null, -- ключ, например относительный filename на диске
  key_md5     uuid primary key, -- по идее уникальный индекс по всем партициям, который обеспечивается софтово
  link        uuid, -- дедубликация, указатель на строчку с content
  links       uuid[], -- куда этот контент указывает
  content     bytea, -- контент ключа
  content_md5 uuid not null, -- md5(content)::uuid что бы не хранить text. Либо от контента, либо от указателя если content is null
  size        integer not null, -- размер. При content is null указывает на контент источника
  created_at  timestamp without time zone default now()
);
create index storage_key_index on storage (key varchar_pattern_ops);
create index storage_content_md5_index on storage (content_md5);
""")
  for k in 'abcdefghijklmnopqrstuvwxyz0123456789':
    plpy.execute("""
create table storage_{0} (like storage including defaults including indexes) inherits (storage);
alter table storage_{0} add constraint constraint_key_md5_storage_{0} check(key_md5::text ~ '^{0}')
""".format(k))
    plpy.execute("""
create or replace function func_reject_insert_on_storage() returns trigger as $trigger$ begin RAISE EXCEPTION 'use import()'; end; $trigger$ language plpgsql;
drop trigger if exists trigger_reject_insert_on_storage on storage;
create trigger trigger_reject_insert_on_storage before insert on storage for each row execute procedure func_reject_insert_on_storage();
""")
$$ language plpythonu;

create or replace function _get_partition_by_uuid(key_md5 text) returns text as $$
  # возвращает партицию для md5 от ключа
  return "storage_{0}".format(key_md5[0])
$$ language plpythonu immutable;

create or replace function _get_partition(key text) returns text as $$
  # возвращает партицию для ключа
  import hashlib
  md5 = hashlib.md5(); md5.update(key); key_md5 = md5.hexdigest()
  plan = plpy.prepare("select _get_partition_by_uuid($1) as partition", ["text"])
  partition = plpy.execute(plan, [key_md5])[0]["partition"]
  return partition
$$ language plpythonu immutable;

create or replace function exist(key text) returns bool as $$
  # ли такая запись с таким ключом в storage
  plan = plpy.prepare("select _get_partition($1) as partition", ["text"])
  partition = plpy.execute(plan, [key])[0]["partition"]
  plan = plpy.prepare("select 1 from {0} where key = $1".format(partition), ["text"])
  rows = plpy.execute(plan, [key])
  return len(rows) > 0
$$ language plpythonu;

create or replace function delete(key text) returns void as $$
  # ли такая запись с таким ключом в storage
  plan = plpy.prepare("select _get_partition($1) as partition", ["text"])
  partition = plpy.execute(plan, [key])[0]["partition"]
  plan = plpy.prepare("select links as links from {0} where key = $1".format(partition), ["text"])
  rows = plpy.execute(plan, [key])
  if len(rows) > 0 and not (rows[0]["links"] is None):
    # в дальнейшем решить данную проблему перевешиванием на другой source:
    plpy.error("links is not empty")
  plan = plpy.prepare("delete from {0} where key = $1".format(partition), ["text"])
  plpy.execute(plan, [key])
$$ language plpythonu;

create or replace function import(key text, path text) returns void as $$
  # вставка данных в storage, в ключ key загружаем данные по пути path
  # дедубликация происходит не на момент вставки, а позднее, бэграундом
  import hashlib, uuid, os
  plan = plpy.prepare("select exist($1) as exist", ["text"])
  if plpy.execute(plan, [key])[0]["exist"]:
    plpy.error("key '{0}' already exists".format(key))
  size = os.path.getsize(path)
  content = open(path, "r").read()
  md5 = hashlib.md5(); md5.update(content); content_md5 = md5.hexdigest()
  md5 = hashlib.md5(); md5.update(key); key_md5 = md5.hexdigest()
  plan = plpy.prepare("select _get_partition_by_uuid($1) as partition", ["text"])
  partition = plpy.execute(plan, [key_md5])[0]["partition"]
  plan = plpy.prepare("insert into {0} (key, content, key_md5, content_md5, size) values ($1, $2, $3, $4, $5)".format(partition), ["text", "bytea", "uuid", "uuid", "integer"])
  plpy.execute(plan, [key, content, uuid.UUID(key_md5), uuid.UUID(content_md5), size])
  content = None
$$ language plpythonu;

create or replace function export(key text, path text) returns void as $$
  # выгружает content по ключу в path
  import os, uuid
  dirname = os.path.dirname(path)
  if not os.path.exists(dirname): os.makedirs(dirname)
  plan = plpy.prepare("select _get_partition($1) as partition", ["text"])
  partition = plpy.execute(plan, [key])[0]["partition"]
  plan = plpy.prepare("select content, link from {0} where key = $1".format(partition), ["text"])
  rows = plpy.execute(plan, [key])
  if len(rows) < 1:
    plpy.error("not found")
  row = rows[0]
  content, link = row["content"], row["link"]
  if not(link is None):
    plan = plpy.prepare("select _get_partition_by_uuid($1) as partition", ["text"])
    partition = plpy.execute(plan, [link])[0]["partition"]
    plan = plpy.prepare("select content from {0} where key_md5 = $1".format(partition), ["uuid"])
    content = plpy.execute(plan, [uuid.UUID(link)])[0]["content"]
  open(path, "w").write(content)
  os.chmod(path, 0o777)
  content = None
$$ language plpythonu;

create or replace function deduplicate() returns void as $$
  # находим все дубли и проставляем у них parent
  import uuid
  rows = plpy.execute("""
select
  array_agg(key_md5) as md5_keys,
  array_agg(link) as links,
  array_agg(case when links is null then false else true end) as is_already_parent
from storage group by content_md5 having count(*) > 1""")

  for row in rows:

    parent, md5_keys = None, []

    # выбираем parent (выбираем то, что никуда не указывает и уже имеет is_already_parent)
    for idx, md5_key in enumerate(row["md5_keys"]):
      if (row["links"][idx] is None) and row["is_already_parent"][idx]:
        parent = md5_key
        break

    # если parent еще не выбран, выбираем первый попавщийся, который сам не явлеться указателем
    if parent is None:
      for idx, md5_key in enumerate(row["md5_keys"]):
        if row["links"][idx] is None:
          parent = md5_key
          break

    if parent is None:
      plpy.notice("parent is not choosed for {0}".format(row["md5_keys"]))
      break

    #собираем md5_keys
    for idx, md5_key in enumerate(row["md5_keys"]):
      if md5_key != parent:
        if row["links"][idx] is None:
          md5_keys.append(md5_key)

    # проверяем что необходимо ли дальше делать
    if len(md5_keys) < 1:
      plpy.notice("nothing to do for {0}".format(row["md5_keys"]))
      break

    plpy.notice("link {0} rows to {1}".format(len(md5_keys), parent))

    # проставляем каждому ключу source
    for key_md5 in md5_keys:
      plan = plpy.prepare("select _get_partition_by_uuid($1) as partition", ["text"])
      partition = plpy.execute(plan, [key_md5])[0]["partition"]
      plan = plpy.prepare("update {0} set link = $1, content = null where key_md5 = $2".format(partition), ["uuid", "uuid"])
      plpy.execute(plan, [uuid.UUID(parent), uuid.UUID(key_md5)])

    # проставляем parent линки
    plan = plpy.prepare("select _get_partition_by_uuid($1) as partition", ["text"])
    partition = plpy.execute(plan, [parent])[0]["partition"]
    uuids = ", ".join(["'{0}'::uuid".format(item) for item in md5_keys])
    plan = plpy.prepare("update {0} set links = ARRAY[{1}]::uuid[] where key_md5 = $1".format(partition, uuids), ["uuid"])
    plpy.execute(plan, [uuid.UUID(parent)])

$$ language plpythonu;
