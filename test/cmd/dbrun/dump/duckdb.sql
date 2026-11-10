-- The fixture of the dump (D291) for DuckDB. The file dump.duckdb holds the
-- schemas core and extra. DuckDB has no trigger, no domain and no stored
-- procedure, so this file has none of them.

-- Types.
create schema core;
create schema extra;

-- An enum, and a type that shares its name with a schema, a table and a column
-- below.
create type core.mood as enum ('sad', 'ok', 'happy');
create type extra.shared as varchar;
create type core.shared as struct(a integer, b varchar);

-- Routines.
-- A macro that reads a table is made after the table, so it is in Dependencies.

-- Tables and indexes.
create sequence core.people_seq start 10 increment 5;
create sequence extra.people_seq;

-- The MAP, UNION and fixed ARRAY types, and a struct that holds a list of
-- maps. The table has a foreign key to its own table and a nextval default.
create table core.people (
  person_id integer primary key default nextval('core.people_seq'),
  full_name varchar not null,
  manager_id integer references core.people (person_id),
  mood core.mood,
  attrs map(varchar, integer),
  contact union(email varchar, phone integer),
  point double[3],
  doc struct(name varchar, tags varchar[], history map(varchar, integer)[]),
  born date,
  seen timestamp,
  score decimal(10, 2),
  note varchar
);
create unique index people_name_idx on core.people (full_name);

insert into core.people (person_id, full_name, manager_id, mood, attrs, contact, point, doc, born, seen, score, note) values
  (1, 'Ana Núñez', null, 'happy', map {'a': 1, 'b': 2}, union_value(email := 'ana@example.com'), [1.0, 2.0, 3.0],
    {'name': 'doc one', 'tags': ['x', 'y'], 'history': [map {'k': 1}, map {'m': 2, 'n': 3}]}, '1990-05-17', '2024-01-01 10:00:00', 9.50, 'the manager');
-- DuckDB checks a foreign key for each statement, so the rows that name a
-- manager come in a second one.
insert into core.people (person_id, full_name, manager_id, mood, attrs, contact, point, doc, born, seen, score, note) values
  (2, 'Zoë "Zed" O''Brien', 1, 'ok', map {}, union_value(phone := 5551234), [0.5, 0.25, 0.125],
    {'name': 'doc two', 'tags': [], 'history': []}, '1985-12-01', null, null, null),
  (3, '李小龍', 1, null, null, null, null, null, null, '2024-03-03 03:03:03', 7.25, 'Unicode: ありがとう');
insert into core.people (person_id, full_name, manager_id, mood, attrs, contact, point, doc, born, seen, score, note) values
  (4, 'Ёлка Müller', 2, 'sad', map {'z': 0}, union_value(email := ''), [0.0, 0.0, 0.0],
    {'name': '', 'tags': ['ü'], 'history': [map {'é': 9}]}, '2001-01-01', '2000-02-29 23:59:59', 0.00, '');

create table extra.people (
  person_id integer primary key default nextval('extra.people_seq'),
  full_name varchar not null,
  mood core.mood,
  attrs map(varchar, integer),
  born date,
  score decimal(10, 2),
  note varchar
);
insert into extra.people (full_name, mood, attrs, born, score, note) values
  ('Bruno Çelik', 'ok', map {'q': 1}, '1999-09-09', 3.50, 'in the second schema'),
  ('Łukasz Żółć', null, null, null, null, null),
  ('田中太郎', 'happy', map {'日': 2}, '2010-10-10', 12.00, 'Unicode: 日本語');

-- Foreign keys that refer to each other. DuckDB refuses a cycle of two
-- tables, so an egg refers to a chicken and the chicken has a column for its
-- first egg that holds no key.
create table core.chicken (
  chicken_id integer primary key,
  first_egg_id integer,
  name varchar
);
create table core.egg (
  egg_id integer primary key,
  chicken_id integer references core.chicken (chicken_id),
  name varchar
);
insert into core.chicken values (1, 1, 'hen'), (2, null, 'Hénrietta');
insert into core.egg values (1, 1, 'first egg'), (2, 2, null);

-- Dependencies.
-- A table macro and a scalar macro that read tables, and views that call them.
create macro core.people_named(pattern) as table
  select person_id, full_name from core.people where full_name like pattern;
create macro core.people_count() as (select count(*) from core.people);
create macro extra.people_count() as (select count(*) from extra.people);

create view core.people_summary as
  select p.person_id, p.full_name, core.people_count() as total
  from core.people p;
-- A view in the other schema that reads core and uses a core macro.
create view extra.both_counts as
  select core.people_count() as core_count, extra.people_count() as extra_count;
create view core.named_a as
  select * from core.people_named('A%');

-- Names.
-- One name for a schema, a table, a column and a type. The type shared is in
-- core and extra above, so this schema, table and column are called shared.
create schema shared;
create table shared.shared (
  shared core.shared,
  other varchar
);
insert into shared.shared values ({'a': 1, 'b': 'one'}, 'x'), ({'a': 2, 'b': 'Ünï'}, null);

-- Mixed case, a space, a reserved word, Unicode and an embedded quote.
create table core."Mixed Case ""Table""" (
  "Order" integer primary key,
  "select" varchar,
  "naïve ✓" varchar,
  "it's" varchar,
  "a ""quoted"" name" double
);
create index "Mixed ""Index""" on core."Mixed Case ""Table""" ("select");
insert into core."Mixed Case ""Table""" values (1, 'from', 'café', 'quote''s', 1.5), (2, null, null, null, null);
create view core."View ""of"" a Table" as
  select "Order", "select" from core."Mixed Case ""Table""";

-- A generated column, which DuckDB computes when it reads one, and a default that is not.
create table core.costs (
  cost_id integer primary key,
  budget decimal(14, 2),
  head_count integer,
  per_head decimal(14, 2) generated always as (budget / nullif(head_count, 0)) virtual,
  label varchar default 'none'
);
insert into core.costs (cost_id, budget, head_count) values (1, 1000, 4), (2, 500, 0), (3, null, 2);

-- Intervals, with months, days and microseconds apart, one in a list, and a negative one.
create table core.intervals (
  id integer primary key,
  every interval,
  gaps interval[]
);
insert into core.intervals values
  (1, interval '1 year 2 months 3 days 4 hours 5 minutes 6.789 seconds', [interval '1 day', interval '2 months']),
  (2, interval '-5 days -1 hour', []),
  (3, null, null);
