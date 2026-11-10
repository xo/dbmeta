-- The dump fixture for MySQL and MariaDB. The two schemas are the databases
-- dump_core and dump_extra, and usql connects to dump_core. A file of the
-- same name that ends in -only holds what one product accepts and the other
-- refuses.

-- Section: types. MySQL has no user types, so a type is a column type.

-- The SET type, with values that need quoting, and an enum on the same table.
create table dump_extra.palette (
  palette_id integer not null auto_increment primary key,
  label varchar(60) not null,
  colors set('red', 'green', 'it''s blue', 'ünï', '黄色') not null default 'red,green',
  shade enum('light', 'dark', 'a b') default 'dark',
  extra_colors set('x', 'y') default null
);
insert into dump_extra.palette (label, colors, shade, extra_colors) values
  ('primary', 'red,green', 'light', 'x,y'),
  ('quoted', 'it''s blue,ünï', 'a b', null),
  ('wide 色', '黄色', null, 'y');

-- Section: tables and indexes.

-- The table people in each schema. The one in dump_core refers to its own
-- table. Every column has a value in some row, and some rows have nulls.
create table dump_core.people (
  person_id integer not null auto_increment primary key,
  boss_id integer,
  full_name varchar(100) character set utf8mb4 collate utf8mb4_bin not null comment 'compared by bytes, so case counts',
  email varchar(254) character set latin1 collate latin1_general_ci,
  born date,
  last_seen datetime(3),
  salary decimal(12, 2),
  rating double,
  active boolean not null default true,
  bio text,
  photo blob,
  mood enum('happy', 'sad', 'ünï') default 'happy',
  skills set('go', 'sql', 'c') default null,
  profile json,
  internal_note varchar(100) invisible,
  unique key people_email_uq (email),
  key people_name_idx (full_name(20), born),
  constraint people_boss_fk foreign key (boss_id) references dump_core.people (person_id) on delete set null on update cascade
);
insert into dump_core.people (person_id, boss_id, full_name, email, born, last_seen, salary, rating, active, bio, photo, mood, skills, profile, internal_note) values
  (1, null, 'Ada Lovelace', 'ada@dump.test', '1815-12-10', '2026-01-02 03:04:05.678', 120000.50, 4.75, true, 'Wrote the first program.', 0x89504e47, 'happy', 'go,sql', '{"role": "founder", "tags": ["math"]}', 'a hidden note'),
  (2, 1, 'Zoë Müller 🎉', 'zoe@dump.test', '1990-02-03', '2026-02-03 04:05:06.001', null, null, false, concat('Line one.', char(10), 'Line two with 日本語.'), null, 'ünï', 'c', null, null),
  (3, 1, 'it''s "quoted" \\ name', null, null, null, 0, 0, true, null, 0x00, null, null, '[]', 'second note');

create table dump_extra.people (
  person_id integer not null auto_increment primary key,
  full_name varchar(100) not null,
  email varchar(254),
  born date,
  note text,
  tags set('a', 'b', 'c'),
  score decimal(8, 3)
);
insert into dump_extra.people (person_id, full_name, email, born, note, tags, score) values
  (1, 'Grace Hopper', 'grace@dump.test', '1906-12-09', 'Found the first bug.', 'a,b', 99.125),
  (2, 'Ünïcödé Ñame', null, null, null, null, null),
  (3, '李雷', 'li@dump.test', '1980-01-01', 'émojis 😀', 'c', -1.5);

-- A character set and a collation on one column, and a column that is
-- invisible. A spatial index needs a column that is not null.
create table dump_core.places (
  place_id integer not null auto_increment primary key,
  place_name varchar(80) character set latin1 collate latin1_german2_ci not null,
  note varchar(80) character set utf8mb4 collate utf8mb4_unicode_ci,
  hidden_code integer invisible default 7,
  location point not null,
  area geometry not null,
  spatial index places_location_idx (location)
);
insert into dump_core.places (place_name, note, hidden_code, location, area) values
  ('Köln', 'Ünï', 1, st_geomfromtext('POINT(6.95 50.94)'), st_geomfromtext('POINT(1 1)')),
  ('Paris', null, null, st_geomfromtext('POINT(2.35 48.85)'), st_geomfromtext('POINT(2 2)'));

-- Foreign keys that refer to each other. The second key is added after both
-- tables exist.
create table dump_core.chicken (
  chicken_id integer not null primary key,
  egg_id integer,
  name varchar(40)
);
create table dump_extra.egg (
  egg_id integer not null primary key,
  chicken_id integer,
  name varchar(40),
  constraint egg_chicken_fk foreign key (chicken_id) references dump_core.chicken (chicken_id)
);
alter table dump_core.chicken
  add constraint chicken_egg_fk foreign key (egg_id) references dump_extra.egg (egg_id);
set foreign_key_checks = 0;
insert into dump_core.chicken values (1, 1, 'Henrietta'), (2, null, 'Ünï');
insert into dump_extra.egg values (1, 1, 'first'), (2, 2, null);
set foreign_key_checks = 1;

-- Section: names. A table that has a name with a space, a reserved word,
-- Unicode, mixed case and a backtick.
create table dump_core.`Mixed Case ``Table`` 表` (
  `select` integer not null,
  `Order By` varchar(40) default 'x y',
  `ünï``cöde` varchar(40),
  `table` text,
  primary key (`select`)
);
insert into dump_core.`Mixed Case ``Table`` 表` values
  (1, 'one', 'ü', 'a `tick`'),
  (2, null, null, null);
create index `idx ``odd`` ünï` on dump_core.`Mixed Case ``Table`` 表` (`Order By`, `ünï``cöde`);

-- One name, thing, for a database, a table, a column, an enum column and an index.
drop database if exists thing;
create database thing character set utf8mb4;
create table thing.thing (
  thing varchar(20) not null,
  kind enum('thing', 'other') not null default 'thing'
);
insert into thing.thing values ('thing', 'thing'), ('x', 'other');
create table dump_core.thing (
  thing enum('thing', 'stuff') not null primary key,
  thing_ref varchar(20),
  key thing (thing_ref)
);
insert into dump_core.thing values ('thing', 'thing'), ('stuff', null);

-- Section: routines.

-- A function that reads a table, which a view calls later.
create function dump_core.person_count() returns integer
  comment 'the number of people in the core schema'
  reads sql data
  return (select count(*) from dump_core.people);

-- DETERMINISTIC and NO SQL, with SQL SECURITY INVOKER and a named definer.
create definer = 'dump_owner'@'localhost' function dump_extra.double_it(n integer) returns integer
  deterministic
  no sql
  sql security invoker
  return n * 2;

create definer = current_user procedure dump_extra.add_person(in new_name varchar(100), out new_id integer)
  modifies sql data
  sql security definer
  comment 'adds a person and returns the key'
begin
  insert into dump_extra.people (full_name) values (new_name);
  set new_id = last_insert_id();
end;

-- A routine takes the sql_mode of the session that creates it. These three
-- are created under different values, and each keeps its own.
set sql_mode = 'ANSI_QUOTES,PIPES_AS_CONCAT,STRICT_ALL_TABLES';
create function dump_core.mode_ansi(a varchar(20), b varchar(20)) returns varchar(50)
  deterministic
  no sql
  return a || '-' || b;
set sql_mode = 'NO_BACKSLASH_ESCAPES,ONLY_FULL_GROUP_BY';
create function dump_core.mode_plain() returns varchar(20)
  deterministic
  no sql
  return 'a\b';
set sql_mode = '';
create function dump_core.mode_empty(n integer) returns decimal(10, 2)
  deterministic
  no sql
  return n / 3;
set sql_mode = default;

-- Section: dependencies.

-- A view that calls a function that reads a table, a view on that view and a
-- view WITH CHECK OPTION.
create view dump_core.people_total as
  select dump_core.person_count() as total, dump_extra.double_it(2) as four;

create view dump_core.active_people as
  select person_id, full_name, active from dump_core.people where active
  with cascaded check option;

create view dump_extra.active_names as
  select full_name from dump_core.active_people;

-- Times, with a value that falls in the hour that a clock skips in New York, a value from the
-- start of the range of a timestamp, and a fraction of a second.
set time_zone = '+00:00';
create table dump_core.stamps (
  stamp_id integer not null primary key,
  seen timestamp null,
  logged datetime,
  precise timestamp(3) null
);
insert into dump_core.stamps values
  (1, '2024-03-10 07:30:00', '2024-03-10 02:30:00', '2024-03-10 07:30:00.123'),
  (2, '1970-01-02 00:00:01', '1948-12-08 00:00:00', '1970-01-02 00:00:01.999'),
  (3, null, null, null);

-- Section: triggers.

create table dump_core.audit (
  audit_id integer not null auto_increment primary key,
  what varchar(100),
  noted_at datetime default current_timestamp
);
create trigger dump_core.people_first before insert on dump_core.people
  for each row set new.full_name = trim(new.full_name);
create trigger dump_core.people_after after update on dump_core.people
  for each row insert into dump_core.audit (what) values (concat('first ', new.person_id));
-- Two more triggers on the same event, ordered with FOLLOWS and PRECEDES.
create trigger dump_core.people_after_log after update on dump_core.people
  for each row follows people_after
  insert into dump_core.audit (what) values (concat('log ', new.person_id));
create trigger dump_core.people_after_pre after update on dump_core.people
  for each row precedes people_after
  insert into dump_core.audit (what) values (concat('pre ', new.person_id));

-- An event that is disabled, so that it does not run while a test reads it.
create event dump_extra.trim_people
  on schedule every 1 day starts '2030-01-01 00:00:00'
  on completion preserve
  disable
  comment 'would remove people with no name'
  do delete from dump_extra.people where full_name = '';

-- Accounts and grants: a table, a view, columns and a whole database. The accounts cannot log in.
CREATE USER IF NOT EXISTS 'dump_reader'@'%' ACCOUNT LOCK;
CREATE USER IF NOT EXISTS 'dump_writer'@'localhost' ACCOUNT LOCK;
GRANT SELECT ON dump_core.people TO 'dump_reader'@'%';
GRANT SELECT, INSERT ON dump_core.audit TO 'dump_reader'@'%' WITH GRANT OPTION;
GRANT SELECT ON dump_core.active_people TO 'dump_reader'@'%';
GRANT SELECT (person_id), UPDATE (full_name) ON dump_core.people TO 'dump_writer'@'localhost';
GRANT SELECT ON dump_extra.* TO 'dump_reader'@'%';
