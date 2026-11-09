-- Objects that Northwind lacks, for the describe commands of usql.

create schema hr;

create type hr.employment_status as enum ('active', 'on_leave', 'contractor', 'retired');
create type hr.email as varchar;

create sequence hr.ticket_seq start 1000 increment 10;
create sequence hr.departments_department_id_seq;
create sequence hr.events_event_id_seq;

create table hr.departments (
  department_id integer primary key default nextval('hr.departments_department_id_seq'),
  code varchar not null unique,
  name varchar not null,
  budget decimal(14, 2) check (budget >= 0),
  head_count smallint not null default 0,
  cost_per_head decimal(14, 2) generated always as (budget / nullif(head_count, 0)) virtual,
  is_active boolean not null default true,
  tags varchar[] not null default [],
  settings json,
  office struct(street varchar, city varchar, postal_code varchar, country varchar),
  network varchar,
  opened_on date,
  review_every interval,
  created_at timestamptz not null default now(),
  updated_at timestamptz
);
comment on table hr.departments is 'the departments of the company';
comment on column hr.departments.cost_per_head is 'budget divided by head count, null with no one';
create index departments_lower_name_idx on hr.departments (lower(name));

-- DuckDB cannot make a foreign key across schemas, so employee_id has none.
create table hr.staff (
  staff_id uuid primary key default uuid(),
  employee_id integer,
  department_id integer not null references hr.departments (department_id),
  email hr.email not null unique,
  status hr.employment_status not null default 'active',
  salary decimal(19, 4),
  rating real,
  photo blob,
  started timestamp,
  ended timestamp,
  notes varchar
);
create index staff_department_idx on hr.staff (department_id, status);

create table hr.events (
  event_id bigint default nextval('hr.events_event_id_seq'),
  happened_at timestamptz not null,
  kind varchar not null,
  detail json,
  primary key (event_id, happened_at)
);
comment on table hr.events is 'an audit log';

create macro hr.department_size(department) as (select count(*) from hr.staff where department_id = department);
comment on macro hr.department_size is 'the number of staff in a department';

create view hr.active_departments as
  select d.code, d.name, d.budget, hr.department_size(d.department_id) as staff
  from hr.departments d
  where d.is_active;

-- DuckDB has no materialized view, so this is a table that holds the rows of the
-- query when the fixture is made.
create table main.sales_by_country as
  select c.country, count(distinct o.order_id) as orders,
    sum(od.unit_price * od.quantity * (1 - od.discount)) as sales
  from customers c
    join orders o on o.customer_id = c.customer_id
    join order_details od on od.order_id = o.order_id
  group by c.country;
comment on table main.sales_by_country is 'the rows of a query, because there is no materialized view';
create unique index sales_by_country_country_idx on main.sales_by_country (country);

insert into hr.departments (code, name, budget, head_count, is_active, tags, settings, office, network, opened_on, review_every) values
  ('SALE', 'Sales', 1250000.00, 4, true, ['revenue', 'field'], '{"quota": 500000, "regions": ["EMEA", "NA"]}', {'street': '1 Market St', 'city': 'Seattle', 'postal_code': '98101', 'country': 'USA'}, '10.1.0.0/16', '1992-04-01', interval '6 months'),
  ('ENGR', 'Engineering', 3400000.50, 3, true, ['product'], '{"oncall": true}', {'street': '2 Kode Way', 'city': 'London', 'postal_code': 'EC1A 1BB', 'country': 'UK'}, '10.2.0.0/16', '1993-01-15', interval '1 year'),
  ('FINC', 'Finance', 420000.00, 1, true, [], null, {'street': '3 Ledger Rd', 'city': 'Tacoma', 'postal_code': '98402', 'country': 'USA'}, null, '1992-04-01', interval '3 months'),
  ('RSCH', 'Research', 0, 0, false, ['closed'], '{"closed": "2001-12-31"}', null, null, '1995-06-01', null),
  ('MKTG', 'Marketing — 市場', 780000.00, 1, true, ['brand', 'social media'], '{"channels": {"tv": 2, "web": 9}}', {'street': '4 Rue Lepic', 'city': 'Paris', 'postal_code': '75018', 'country': 'France'}, '10.4.0.0/16', '1996-09-30', interval '2 months 15 days');

update hr.departments set budget = budget * 1.05 where code = 'SALE';

insert into hr.staff (employee_id, department_id, email, status, salary, rating, started, ended, notes) values
  (1, 1, 'nancy.davolio@northwind.test', 'active', 62000, 4.5, '1992-05-01', null, 'Top seller of 1997.'),
  (2, 1, 'andrew.fuller@northwind.test', 'active', 98000, 4.9, '1992-08-14', null, null),
  (3, 1, 'janet.leverling@northwind.test', 'on_leave', 59000, 4.1, '1992-04-01', null, E'Line one.\nLine two.'),
  (4, 1, 'margaret.peacock@northwind.test', 'active', 61000, null, '1993-05-03', null, null),
  (5, 2, 'steven.buchanan@northwind.test', 'active', 105000, 3.8, '1993-10-17', null, 'Leads the UK office.'),
  (6, 2, 'michael.suyama@northwind.test', 'contractor', null, 3.2, '1993-10-17', '1999-01-01', null),
  (7, 2, 'robert.king@northwind.test', 'active', 88000, 4.0, '1994-01-02', null, null),
  (8, 3, 'laura.callahan@northwind.test', 'active', 54000, 4.4, '1994-03-05', null, 'Inside sales coordinator.'),
  (9, 5, 'anne.dodsworth@northwind.test', 'retired', 0, 5.0, '1994-11-15', '2020-06-30', 'Ünïcödé, émojis 🎉 and a long note that goes on to be wider than most terminals are, so that the wrapped and aligned formats have something to wrap.');

-- DuckDB has no trigger, so the log that the triggers of the other products
-- write is written here.
insert into hr.events (happened_at, kind, detail)
  select now(), 'insert', json_object('staff_id', staff_id) from hr.staff;
