-- The dump fixture for Oracle. usql opens it as SYSTEM with the current schema
-- set to dump_core. A name with no prefix is in dump_core and every name in
-- dump_extra has its prefix. The two users are made before this file loads.
-- Oracle has one name space for the tables and the types of a user, so the name
-- shape is a type and a column, the table is named shapes, and no user has the
-- name. That is the nearest object.

-- Section: types
create type person_t as object (
  name varchar2(100),
  born date,
  member function age_on (day date) return number,
  member function label return varchar2
) not final;
create type body person_t as
  member function age_on (day date) return number is
  begin
    return trunc(months_between(day, born) / 12);
  end;
  member function label return varchar2 is
  begin
    return name;
  end;
end;
create type employee_t under person_t (
  salary number(12, 2),
  overriding member function label return varchar2
);
create type body employee_t as
  overriding member function label return varchar2 is
  begin
    return name || ' (' || salary || ')';
  end;
end;
create type phone_list as varray(5) of varchar2(30);
create type tag_table as table of varchar2(40);
create type address_t as object (street varchar2(100), city varchar2(60));
create type shape as object (shape varchar2(30));
create type score_row as object (id number, score number);
create type score_table as table of score_row;
create type dump_extra_note as object (text varchar2(200));
create type dump_extra.extra_t as object (code varchar2(10));
grant execute on dump_extra.extra_t to dump_core;

-- Section: tables and indexes
create table people (
  id number(10) primary key,
  name varchar2(100) not null,
  boss_id number(10),
  email varchar2(200),
  born date,
  score number(8, 2),
  bio clob,
  joined timestamp with time zone,
  constraint people_boss_fk foreign key (boss_id) references people (id)
);
insert into people values (1, 'Ann Ünder', null, 'ann@example.com', date '1980-02-29', 91.5, 'Ann leads the team. 日本語', timestamp '2020-01-01 09:00:00 +00:00');
insert into people values (2, 'Bob', 1, null, null, null, null, null);
insert into people values (3, 'Çelik ''Quote''', 1, 'celik@example.com', date '1990-12-31', 7, 'Ünïcödé ☃', timestamp '2021-06-30 23:59:59 -05:00');
insert into people values (4, 'Dmitri Дмитрий', 2, 'd@example.com', date '1975-07-04', 0, 'Привет мир', timestamp '2019-03-03 03:03:03 +01:00');
commit;

create table dump_extra.people (
  id number(10) primary key,
  name varchar2(100) not null,
  core_id number(10),
  email varchar2(200),
  born date,
  score number(8, 2),
  bio clob,
  joined timestamp with time zone
);
insert into dump_extra.people values (1, 'Extra Émile', 1, 'emile@example.com', date '2000-01-01', 12.25, 'Émile 😀', timestamp '2022-02-02 02:02:02 +00:00');
insert into dump_extra.people values (2, 'Extra Zoë', null, null, null, null, null, null);
commit;
grant references, select on people to dump_extra;
grant references, select on dump_extra.people to dump_core;

create table shapes (shape shape, label varchar2(30));
insert into shapes values (shape('round'), 'circle');
insert into shapes values (null, null);
commit;

-- an invisible column and a deferrable foreign key
create table pets (
  id number(10) primary key,
  owner_id number(10),
  name varchar2(60),
  secret varchar2(60) invisible,
  constraint pets_owner_fk foreign key (owner_id) references people (id) deferrable initially deferred
);
insert into pets (id, owner_id, name, secret) values (1, 1, 'Rex', 'bone');
commit;

-- foreign keys that refer to each other
create table departments (
  id number(10) primary key,
  name varchar2(60),
  head_id number(10)
);
create table staff (
  id number(10) primary key,
  name varchar2(60),
  department_id number(10) references departments (id)
);
alter table departments add constraint departments_head_fk foreign key (head_id) references staff (id) deferrable initially deferred;
insert into departments values (1, 'Research', null);
insert into staff values (1, 'Ann', 1);
update departments set head_id = 1;
commit;

-- a foreign key from dump_core to dump_extra
create table ledger (
  id number(10) primary key,
  extra_id number(10) references dump_extra.people (id),
  amount number(10, 2)
);
insert into ledger values (1, 1, 10.5);
commit;

-- a nested table column with STORE AS, a varray column and a REF column
create table addresses of address_t;
insert into addresses values (address_t('1 Main St', 'Zürich'));
create table contacts (
  id number(10) primary key,
  tags tag_table,
  phones phone_list,
  home ref address_t,
  who employee_t
) nested table tags store as contacts_tags_nt;
insert into contacts select 1, tag_table('a', 'ü'), phone_list('1', '2'), ref(a), employee_t('Eve', date '1999-09-09', 5000) from addresses a;
commit;

-- an index organized table
create table lookup (
  code varchar2(10),
  label varchar2(60),
  constraint lookup_pk primary key (code)
) organization index;
insert into lookup values ('A', 'alpha');
insert into lookup values ('B', 'beta');
commit;

-- a sequence, and a default that reads it
create sequence ticket_seq start with 100 increment by 5 cache 5;
create table tickets (
  id number(10) default ticket_seq.nextval primary key,
  title varchar2(60),
  created timestamp default systimestamp
);
insert into tickets (title) values ('first');
commit;

-- Section: routines
-- a package with overloads. The spec must load before the view that calls it.
create package calc as
  function area (side number) return number;
  function area (width number, height number) return number;
  procedure bump (value in out number, step number default 1);
  function people_count return number;
end calc;
create package body calc as
  function area (side number) return number is
  begin
    return side * side;
  end;
  function area (width number, height number) return number is
  begin
    return width * height;
  end;
  procedure bump (value in out number, step number default 1) is
  begin
    value := value + step;
  end;
  function people_count return number is
    n number;
  begin
    select count(*) into n from people;
    return n;
  end;
end calc;

-- a deterministic function that an index and a virtual column use
create function double_it (n number) return number deterministic as
begin
  return n * 2;
end;
create table measures (
  id number(10) primary key,
  value number(10),
  doubled number generated always as (double_it(value)) virtual
);
insert into measures (id, value) values (1, 21);
insert into measures (id, value) values (2, null);
commit;
create index measures_double_idx on measures (double_it(value));

-- AUTHID CURRENT_USER
create function who_am_i return varchar2 authid current_user as
begin
  return sys_context('USERENV', 'CURRENT_USER');
end;

-- a pipelined table function
create function scores return score_table pipelined as
begin
  for r in (select id, score from people where score is not null order by id) loop
    pipe row (score_row(r.id, r.score));
  end loop;
  return;
end;

-- a function with a type in its signature, and a procedure that calls it
create function address_city (a address_t) return varchar2 as
begin
  return a.city;
end;
create procedure rename_person (p_id number, p_name varchar2) as
begin
  update people set name = p_name where id = p_id;
end;

-- a function in dump_extra, called from dump_core
create function dump_extra.twice (n number) return number as
begin
  return n + n;
end;
grant execute on dump_extra.twice to dump_core;

-- Section: triggers
create table audit_log (id number generated always as identity, what varchar2(100), at timestamp default systimestamp);
create view people_names as select id, name from people;

-- a statement trigger
create trigger people_stmt after insert on people
begin
  insert into audit_log (what) values ('insert on people');
end;

-- UPDATE OF and WHEN
create trigger people_email_upd before update of email on people
  for each row
  when (new.email is not null)
begin
  :new.email := lower(:new.email);
end;

-- INSTEAD OF on a view
create trigger people_names_ins instead of insert on people_names
  for each row
begin
  insert into people (id, name) values (:new.id, :new.name);
end;

-- a compound trigger. Known skip: usql does not read a compound trigger as one
-- unit and cuts it at the first semicolon, even when the semicolon is escaped.
-- An anonymous block that creates it with EXECUTE IMMEDIATE loads whole, so the
-- trigger is the same and only the way it is sent differs.
begin
  execute immediate q'[create trigger pets_compound
  for insert or update on pets
  compound trigger
  n number := 0;
  before statement is
  begin
    n := 0;
  end before statement;
  after each row is
  begin
    n := n + 1;
  end after each row;
  after statement is
  begin
    insert into audit_log (what) values ('pets changed ' || n);
  end after statement;
end pets_compound;]';
end;

-- a schema DDL trigger
create trigger ddl_audit after alter on schema
begin
  insert into audit_log (what) values ('alter ' || ora_dict_obj_name);
end;

-- Section: dependencies
-- a view that calls a package function, and a materialized view log
create view people_stats as
  select count(*) as rows_in_table, calc.people_count as rows_by_package, calc.area(3, 4) as area
  from dual;
create materialized view log on people with primary key;
create view cross_view as
  select p.id, p.name, e.name as extra_name
  from people p join dump_extra.people e on e.core_id = p.id;
create view extra_twice as select id, dump_extra.twice(id) as twice from people;
create synonym extra_people for dump_extra.people;

-- Section: names
-- mixed case, a space, a reserved word and Unicode. Oracle refuses a double
-- quote inside a name, so the embedded quote is a single quote, the nearest name.
create table "Mixed Case" (
  "Id" number(10) primary key,
  "select" varchar2(30),
  "from" varchar2(30),
  "Ünï cödé" varchar2(30),
  "say 'hi'" varchar2(30)
);
insert into "Mixed Case" values (1, 'a', 'b', 'c', 'd');
insert into "Mixed Case" values (2, null, null, null, null);
commit;
create view "Mixed View" as select "Id", "select" from "Mixed Case";
create sequence "Seq Ünï";
create table "a'b" (id number);
