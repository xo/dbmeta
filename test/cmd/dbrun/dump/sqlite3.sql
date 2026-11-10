-- The fixture of the dump (D291) for SQLite. dbmeta reads the schema main of
-- SQLite and no attached one, so the file is one schema, and the tables of the
-- second schema have the prefix extra_.
-- SQLite has no type, no function that a user writes and no schema that a
-- foreign key can cross, and each section says what stands in for them.

-- Types.
-- SQLite has no user type, no enum and no domain. A STRICT table is the
-- nearest thing, because it refuses a value of the wrong type, and a check
-- constraint stands in for an enum.

-- Routines.
-- SQLite has no stored function and no procedure, so a view that calls a
-- function that reads a table cannot exist. The view below reads the same
-- table with a correlated subquery. A function in a signature has no
-- counterpart either.

-- Tables and indexes.

-- The first schema is main. The table has a foreign key to itself, which is
-- deferred, so that a row can name a manager that comes later in the dump.
create table main.people (
  person_id integer primary key,
  full_name text not null,
  manager_id integer references people (person_id) deferrable initially deferred,
  score real,
  photo blob,
  born text,
  note text
);
create index main.people_name_idx on people (full_name);
insert into main.people (person_id, full_name, manager_id, score, photo, born, note) values
  (1, 'Ana Núñez', null, 9.5, x'00ff10', '1990-05-17', 'the manager'),
  (2, 'Zoë "Zed" O''Brien', 1, null, null, '1985-12-01', null),
  (3, '李小龍', 1, 7.25, x'cafe', null, 'Unicode: ありがとう'),
  (4, 'Ёлка Müller', 2, 0, x'', '2001-01-01', ''),
  (5, 'Plain Person', 3, -1.5, x'01', '1970-07-04', 'last');

-- The second schema is the tables that start with extra_.
create table extra_people (
  person_id integer primary key,
  full_name text not null,
  score real,
  photo blob,
  born text,
  note text
);
create index extra_people_name_idx on extra_people (full_name);
insert into extra_people (person_id, full_name, score, photo, born, note) values
  (1, 'Bruno Çelik', 3.5, x'aa', '1999-09-09', 'in the second file'),
  (2, 'Łukasz Żółć', null, null, null, null),
  (3, '田中太郎', 12, x'bb00', '2010-10-10', 'Unicode: 日本語');

-- A column with no type, which keeps a value as it is given, and a table that
-- CREATE TABLE AS SELECT made, whose columns take the types of their expressions.
create table main.untyped (id integer primary key, orders, sales, label text);
insert into untyped (id, orders, sales, label) values (1, 3, 9.5, 'a'), (2, 'many', null, 'b');
create table main.made_from as
  select person_id, person_id + 1 as next_id, full_name || '!' as shout, score * 2 as double_score, person_id * 1.5 as ratio, x'01' as blob_col, 1 as untyped_literal
  from people;

-- A STRICT table with every type that it allows, and a check that stands in
-- for an enum.
create table main.strict_kinds (
  kind_id integer primary key,
  kind text not null check (kind in ('small', 'medium', 'large')) default 'medium',
  amount integer not null default 0,
  ratio real,
  label text,
  payload blob,
  anything any
) strict;
insert into main.strict_kinds (kind_id, kind, amount, ratio, label, payload, anything) values
  (1, 'small', 1, 0.5, 'one', x'01', 'text'),
  (2, 'large', 2, null, null, null, 42),
  (3, 'medium', 3, 3.25, 'Ünïcödé ✓', x'fffe', null);

-- A table WITHOUT ROWID, with a composite key.
create table main.pairs (
  left_key text not null,
  right_key integer not null,
  weight real,
  note text,
  primary key (left_key, right_key)
) without rowid;
insert into main.pairs (left_key, right_key, weight, note) values
  ('a', 1, 1.5, 'first'),
  ('é', 2, null, null),
  ('日本', 3, 2.5, 'wide');

-- AUTOINCREMENT puts a row in sqlite_sequence, and the dump must keep it. The
-- last row is removed, so that the sequence is ahead of the table.
create table main.tickets (
  ticket_id integer primary key autoincrement,
  title text not null,
  opened text,
  closed text
);
insert into main.tickets (title, opened, closed) values
  ('first', '2024-01-01', '2024-01-02'),
  ('second', '2024-02-01', null),
  ('third ✓', null, null),
  ('removed', '2024-04-01', null);
delete from main.tickets where title = 'removed';

-- Foreign keys that refer to each other. Both are deferred, because the rows
-- of one need the rows of the other.
create table main.chicken (
  chicken_id integer primary key,
  egg_id integer references egg (egg_id) deferrable initially deferred,
  name text
);
create table main.egg (
  egg_id integer primary key,
  chicken_id integer references chicken (chicken_id) deferrable initially deferred,
  name text
);
insert into main.chicken (chicken_id, egg_id, name) values (1, 1, 'hen'), (2, null, 'Hénrietta');
insert into main.egg (egg_id, chicken_id, name) values (1, 1, 'first egg'), (2, 2, null);

-- Triggers.
-- A view for the INSTEAD OF trigger.
create view main.ticket_titles as
  select ticket_id, title, (select count(*) from main.people) as people_count
  from main.tickets;

create table main.audit (
  audit_id integer primary key,
  what text not null,
  detail text
);

-- An INSTEAD OF trigger on a view.
create trigger main.ticket_titles_insert instead of insert on ticket_titles
begin
  insert into tickets (title) values (new.title);
  insert into audit (what, detail) values ('insert', new.title);
end;

-- A trigger with a WHEN condition.
create trigger main.people_note_added after insert on people
when new.note is not null and length(new.note) > 20
begin
  insert into audit (what, detail) values ('long note', new.full_name);
end;

-- A trigger on UPDATE OF one column.
create trigger main.people_score_changed after update of score on people
begin
  insert into audit (what, detail) values ('score', old.score || ' to ' || new.score);
end;

-- Rows that the triggers wrote.
insert into main.audit (what, detail) values
  ('seed', 'a row with Unicode ✓'),
  ('seed', null);

-- An FTS5 table makes hidden tables of its own, and a dump must not create
-- them again. Known skip: the SQLite that usql ships is built without FTS5, so
-- the statement stays here as a comment.
-- create virtual table main.documents using fts5(title, body);
-- insert into documents (title, body) values ('first', 'the body of the first'), ('Ünï', 'second ✓');

-- Dependencies.
-- SQLite refuses a view that reads another schema, so each schema has a view
-- of its own table instead.
create view main.all_people as
  select 'main' as source, person_id, full_name from main.people
  union all
  select 'chicken', chicken_id, name from main.chicken;
create view extra_people_names as
  select person_id, full_name from extra_people;

-- A view on a view.
create view main.big_ticket_titles as
  select * from main.ticket_titles where ticket_id > 1;

-- Names.
-- One name for a table and a column. The table and its column are called extra,
-- like the prefix of the second schema. SQLite has no type to name.
create table main."extra" (
  "extra" text not null primary key,
  other text
);
insert into main."extra" ("extra", other) values ('x', 'one'), ('Ünï', null);

-- Mixed case, a space, a reserved word, Unicode and an embedded quote.
create table main."Mixed Case ""Table""" (
  "Order" integer primary key,
  "select" text,
  "naïve ✓" text,
  "it's" text,
  "a ""quoted"" name" real
);
create index main."Mixed ""Index""" on "Mixed Case ""Table""" ("select", "naïve ✓");
insert into main."Mixed Case ""Table""" ("Order", "select", "naïve ✓", "it's", "a ""quoted"" name") values
  (1, 'from', 'café', 'quote''s', 1.5),
  (2, null, null, null, null);
create view main."View ""of"" a Table" as
  select "Order", "select" from main."Mixed Case ""Table""";

-- A unique index that a person named, and a check inside a table.
create unique index people_name_unique on people (full_name);
create table checked (
  id integer primary key,
  qty integer not null check (qty > 0),
  kind text check (kind in ('a', 'b'))
);
insert into checked (id, qty, kind) values (1, 5, 'a'), (2, 7, null);
