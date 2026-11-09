-- Types: two schemas, a name that a schema, a table, a column and a type all
-- use, an alias type, a sequence and an XML schema collection.

create schema core;
create schema extra;
create schema label;

create type core.email from nvarchar(254) not null;
create type label.label from nvarchar(60) null;
create type core.name_list as table (name nvarchar(100) primary key);

create sequence extra.person_seq as int start with 100 increment by 5;

create xml schema collection extra.person_xsd as N'<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="profile">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="city" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>';

-- Routines that tables use: a scalar function that a computed column calls,
-- and one with a type in its signature.

create function core.fn_age_band(@age int)
returns varchar(8)
with schemabinding
as
begin
  return case when @age is null then 'unknown' when @age < 18 then 'minor' when @age < 65 then 'adult' else 'senior' end;
end;

create function core.fn_label(@text label.label)
returns core.email
as
begin
  return lower(replace(@text, N' ', N'.')) + N'@example.org';
end;

-- Tables and indexes: a foreign key to its own table, constraints with names,
-- a default that reads a sequence, an XML column, and the indexes.

create table core.people (
  id int identity(1, 1) not null constraint pk_core_people primary key,
  parent_id int null constraint fk_core_people_parent references core.people (id),
  name nvarchar(100) not null,
  email core.email null,
  age int null constraint ck_core_people_age check (age is null or age between 0 and 150),
  age_band as core.fn_age_band(age),
  born date null,
  score decimal(8, 2) null,
  active bit not null constraint df_core_people_active default 1,
  note nvarchar(max) null,
  created datetime2(3) not null constraint df_core_people_created default '2024-01-02T03:04:05.678'
);

create table extra.people (
  id int not null constraint df_extra_people_id default (next value for extra.person_seq) constraint pk_extra_people primary key,
  name nvarchar(100) not null,
  email nvarchar(254) null,
  age int null,
  score decimal(8, 2) null,
  active bit not null constraint df_extra_people_active default 1,
  note nvarchar(max) null,
  profile xml(extra.person_xsd) null
);

create nonclustered index ix_core_people_active on core.people (name) include (email, score) where active = 1;
create nonclustered columnstore index cs_extra_people on extra.people (age, score);
create index ix_extra_people_name on extra.people (name desc) include (email);

-- Dependencies: two tables with foreign keys that refer to each other, one in
-- each schema, a view that calls a function that reads a table, and a view in
-- the other schema that uses a function with schemabinding.

create table core.dept (
  id int not null constraint pk_core_dept primary key,
  org_id int null
);
create table extra.org (
  id int not null constraint pk_extra_org primary key,
  dept_id int null constraint fk_extra_org_dept references core.dept (id)
);
alter table core.dept add constraint fk_core_dept_org foreign key (org_id) references extra.org (id);

create function extra.fn_people_count()
returns int
as
begin
  return (select count(*) from extra.people);
end;

create view core.v_people_count as
select extra.fn_people_count() as total;

create function extra.fn_adults(@min int)
returns table
with schemabinding
as
return (select id, name, age from extra.people where age >= @min);

create view core.v_adults as
select id, name, age from extra.fn_adults(18);

create function core.fn_people_by_parent(@parent int)
returns table
as
return (select id, name, email from core.people where parent_id = @parent);

create function core.fn_names_upper(@min int)
returns @result table (id int, upper_name nvarchar(100))
as
begin
  insert into @result select id, upper(name) from core.people where id >= @min;
  return;
end;

-- Names: mixed case, a space, a reserved word, Unicode, an embedded quote and
-- a closing bracket. The name label is a schema, a table, a column and a type.

create table core.[Mixed Case "Select"] (
  [Select] int not null constraint [pk_Mixed Case] primary key,
  [it's] nvarchar(50) null,
  [a]]b] nvarchar(50) null,
  [Ünï çödé ✓] nvarchar(50) null
);

create table core.[label] (
  id int not null constraint pk_core_label primary key,
  [label] label.label
);

create table extra.[Order] (
  [Group] int not null constraint pk_extra_order primary key,
  [Table] nvarchar(50) null
);

-- Rows.

set identity_insert core.people on;
insert into core.people (id, parent_id, name, email, age, born, score, active, note, created) values
  (1, null, N'Ada Lovelace', N'ada@example.org', 36, '1815-12-10', 99.50, 1, N'first', '2024-05-06T07:08:09.123'),
  (2, 1, N'Zoë Müller', N'zoe@example.org', 12, '2012-03-04', null, 1, N'Unicode: ñ ü 日本語 ✓', '2024-05-07T07:08:09.123'),
  (3, 1, N'O''Brien', N'ob@example.org', null, null, 0.00, 0, null, '2024-05-08T07:08:09.123'),
  (4, 2, N'Ünal Çelik', N'unal@example.org', 70, '1950-01-31', 12.34, 1, N'', '2024-05-09T07:08:09.123');
set identity_insert core.people off;

insert into extra.people (name, email, age, score, active, note, profile) values
  (N'Grace Hopper', N'grace@example.org', 85, 88.80, 1, N'admiral', N'<profile><city>New York</city></profile>'),
  (N'Zoë Müller', null, 12, null, 0, N'日本語 ✓', null),
  (N'O''Neil', N'on@example.org', null, 1.25, 1, null, N'<profile><city>Dublin</city></profile>');

insert into core.dept (id, org_id) values (1, null);
insert into extra.org (id, dept_id) values (1, 1);
update core.dept set org_id = 1 where id = 1;
insert into core.[Mixed Case "Select"] values (1, N'it''s', N'a]b', N'Ünï çödé ✓'), (2, null, null, null);
insert into core.[label] values (1, N'tag'), (2, null);
insert into extra.[Order] values (1, N'T'), (2, null);

-- Procedures: OUTPUT parameters, parameter defaults, and a procedure of
-- several statements with no BEGIN and END around them. EXEC sends the last
-- one as one string, because usql ends a statement at each semicolon.

create procedure core.sp_count_people
  @min_age int = 0,
  @total int output,
  @label label.label = N'all'
as
begin
  select @total = count(*) from core.people where isnull(age, 0) >= @min_age;
end;

create procedure extra.sp_rename_person
  @id int,
  @name nvarchar(100) = N'Anonymous',
  @old nvarchar(100) = null output
as
begin
  select @old = name from extra.people where id = @id;
  update extra.people set name = @name where id = @id;
end;

exec('create procedure core.sp_no_begin @min int = 0 as
  select id, name from core.people where id >= @min;
  select count(*) as total from core.people;
  select max(age) as oldest from core.people');

-- Triggers: AFTER triggers with an order set, a disabled trigger, INSTEAD OF on
-- a view, and a trigger on the database for DDL.

create table core.audit (
  id int identity(1, 1) not null constraint pk_core_audit primary key,
  what nvarchar(50) not null
);

create trigger core.trg_people_first on core.people after insert as
begin
  set nocount on;
  insert into core.audit (what) select N'first ' + name from inserted;
end;

create trigger core.trg_people_last on core.people after insert as
begin
  set nocount on;
  insert into core.audit (what) select N'last ' + name from inserted;
end;

create trigger core.trg_people_off on core.people after update as
begin
  set nocount on;
  insert into core.audit (what) select N'update ' + name from inserted;
end;
disable trigger core.trg_people_off on core.people;

create view core.v_people_names as
select id, name from core.people;

create trigger core.trg_names_instead on core.v_people_names instead of insert as
begin
  set nocount on;
  insert into core.people (name) select name from inserted;
end;

exec sp_settriggerorder @triggername = N'core.trg_people_first', @order = N'First', @stmttype = N'INSERT';
exec sp_settriggerorder @triggername = N'core.trg_people_last', @order = N'Last', @stmttype = N'INSERT';

create trigger trg_ddl_dump on database for create_table as
begin
  set nocount on;
end;

-- Security: a policy that filters the rows of extra.people with an inline
-- function that has schemabinding.

create function extra.fn_rls(@id int)
returns table
with schemabinding
as
return (select 1 as allowed where @id > 0);

create security policy extra.people_policy
  add filter predicate extra.fn_rls(id) on extra.people
  with (state = on);
