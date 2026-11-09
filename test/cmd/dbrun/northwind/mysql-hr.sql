-- Objects that Northwind lacks, for the describe commands of usql. MySQL and
-- MariaDB both read this file. A schema is a database here.

create database hr character set utf8mb4;

create table hr.departments (
  department_id integer not null auto_increment primary key,
  code char(4) not null unique,
  name varchar(100) not null,
  budget decimal(14, 2) check (budget >= 0),
  head_count smallint not null default 0,
  cost_per_head decimal(14, 2) generated always as (budget / nullif(head_count, 0)) stored comment 'budget divided by head count, null with no one',
  is_active boolean not null default true,
  tags json not null default (json_array()),
  settings json,
  office_street varchar(100),
  office_city varchar(60),
  office_postal_code varchar(20),
  office_country varchar(60),
  network varchar(43),
  opened_on date,
  review_every varchar(30),
  created_at timestamp not null default current_timestamp,
  updated_at timestamp null default null
) comment 'the departments of the company';
create index departments_active_idx on hr.departments (is_active, name);

create table hr.staff (
  staff_id char(36) not null default (uuid()) primary key,
  employee_id integer,
  department_id integer not null,
  email varchar(254) not null unique,
  status enum('active', 'on_leave', 'contractor', 'retired') not null default 'active',
  salary decimal(19, 4),
  rating float,
  photo blob,
  started datetime,
  ended datetime,
  notes text,
  constraint staff_email_chk check (email regexp '^[^@ ]+@[^@ ]+$'),
  constraint staff_employee_fk foreign key (employee_id) references northwind.employees (employee_id),
  constraint staff_department_fk foreign key (department_id) references hr.departments (department_id) on delete cascade
);
create index staff_department_idx on hr.staff (department_id, status);

create table hr.events (
  event_id bigint not null auto_increment,
  happened_at datetime not null,
  kind varchar(20) not null,
  detail json,
  primary key (event_id, happened_at)
) comment 'an audit log, one partition for each year'
partition by range columns (happened_at) (
  partition events_2025 values less than ('2026-01-01'),
  partition events_2026 values less than ('2027-01-01')
);

create trigger hr.departments_touch before update on hr.departments
  for each row set new.updated_at = current_timestamp;

create trigger hr.staff_log_insert after insert on hr.staff
  for each row insert into hr.events (happened_at, kind, detail)
  values (now(), 'insert', json_object('staff_id', new.staff_id));
create trigger hr.staff_log_update after update on hr.staff
  for each row insert into hr.events (happened_at, kind, detail)
  values (now(), 'update', json_object('staff_id', new.staff_id));
create trigger hr.staff_log_delete after delete on hr.staff
  for each row insert into hr.events (happened_at, kind, detail)
  values (now(), 'delete', json_object('staff_id', old.staff_id));

create procedure hr.give_raise(department integer, percent decimal(7, 2))
  update hr.staff set salary = salary * (1 + percent / 100) where department_id = department;

create function hr.department_size(department integer) returns bigint
  comment 'the number of staff in a department'
  reads sql data
  return (select count(*) from hr.staff where department_id = department);

create view hr.active_departments as
  select d.code, d.name, d.budget, hr.department_size(d.department_id) as staff
  from hr.departments d
  where d.is_active;

-- MySQL has no materialized view, so this is a table that holds the rows of the
-- query when the fixture is made.
create table sales_by_country comment 'the rows of a query, because there is no materialized view' as
  select c.country, count(distinct o.order_id) as orders,
    sum(od.unit_price * od.quantity * (1 - od.discount)) as sales
  from customers c
    join orders o on o.customer_id = c.customer_id
    join order_details od on od.order_id = o.order_id
  group by c.country;
create unique index sales_by_country_country_idx on sales_by_country (country);
create fulltext index products_name_ft_idx on products (product_name);

create role if not exists northwind_reader;
grant select on northwind.* to northwind_reader;
grant select on hr.* to northwind_reader;

insert into hr.departments (code, name, budget, head_count, is_active, tags, settings, office_street, office_city, office_postal_code, office_country, network, opened_on, review_every) values
  ('SALE', 'Sales', 1250000.00, 4, true, '["revenue", "field"]', '{"quota": 500000, "regions": ["EMEA", "NA"]}', '1 Market St', 'Seattle', '98101', 'USA', '10.1.0.0/16', '1992-04-01', '6 months'),
  ('ENGR', 'Engineering', 3400000.50, 3, true, '["product"]', '{"oncall": true}', '2 Kode Way', 'London', 'EC1A 1BB', 'UK', '10.2.0.0/16', '1993-01-15', '1 year'),
  ('FINC', 'Finance', 420000.00, 1, true, '[]', null, '3 Ledger Rd', 'Tacoma', '98402', 'USA', null, '1992-04-01', '3 months'),
  ('RSCH', 'Research', 0, 0, false, '["closed"]', '{"closed": "2001-12-31"}', null, null, null, null, null, '1995-06-01', null),
  ('MKTG', 'Marketing — 市場', 780000.00, 1, true, '["brand", "social media"]', '{"channels": {"tv": 2, "web": 9}}', '4 Rue Lepic', 'Paris', '75018', 'France', '10.4.0.0/16', '1996-09-30', '2 mons 15 days');

insert into hr.staff (employee_id, department_id, email, status, salary, rating, started, ended, notes) values
  (1, 1, 'nancy.davolio@northwind.test', 'active', 62000, 4.5, '1992-05-01', null, 'Top seller of 1997.'),
  (2, 1, 'andrew.fuller@northwind.test', 'active', 98000, 4.9, '1992-08-14', null, null),
  (3, 1, 'janet.leverling@northwind.test', 'on_leave', 59000, 4.1, '1992-04-01', null, concat('Line one.', char(10), 'Line two.')),
  (4, 1, 'margaret.peacock@northwind.test', 'active', 61000, null, '1993-05-03', null, null),
  (5, 2, 'steven.buchanan@northwind.test', 'active', 105000, 3.8, '1993-10-17', null, 'Leads the UK office.'),
  (6, 2, 'michael.suyama@northwind.test', 'contractor', null, 3.2, '1993-10-17', '1999-01-01', null),
  (7, 2, 'robert.king@northwind.test', 'active', 88000, 4.0, '1994-01-02', null, null),
  (8, 3, 'laura.callahan@northwind.test', 'active', 54000, 4.4, '1994-03-05', null, 'Inside sales coordinator.'),
  (9, 5, 'anne.dodsworth@northwind.test', 'retired', 0, 5.0, '1994-11-15', '2020-06-30', 'Ünïcödé, émojis 🎉 and a long note that goes on to be wider than most terminals are, so that the wrapped and aligned formats have something to wrap.');

update hr.departments set budget = budget * 1.05 where code = 'SALE';
