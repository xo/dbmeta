-- Objects that Northwind lacks, for the describe commands of usql. SQLite has
-- no schema that a table can reference across, so the names start with hr_.

create table hr_departments (
  department_id integer primary key autoincrement,
  code char(4) not null unique,
  name text not null,
  budget numeric(14, 2) check (budget >= 0),
  head_count smallint not null default 0,
  cost_per_head numeric(14, 2) generated always as (budget / nullif(head_count, 0)) stored,
  is_active boolean not null default 1,
  tags text not null default '[]' check (json_valid(tags)),
  settings text check (settings is null or json_valid(settings)),
  office_street text,
  office_city text,
  office_postal_code text,
  office_country text,
  network text,
  opened_on date,
  review_every text,
  created_at timestamp not null default current_timestamp,
  updated_at timestamp
);
create index hr_departments_active_idx on hr_departments (name) where is_active;
create index hr_departments_lower_name_idx on hr_departments (lower(name));

create table hr_staff (
  staff_id text primary key default (lower(hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-4' || substr(hex(randomblob(2)), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(hex(randomblob(2)), 2) || '-' || hex(randomblob(6)))),
  employee_id integer references employees (employee_id),
  department_id integer not null references hr_departments (department_id) on delete cascade,
  email text not null unique check (email glob '*?@?*' and email not glob '* *'),
  status text not null default 'active' check (status in ('active', 'on_leave', 'contractor', 'retired')),
  salary numeric(19, 4),
  rating real,
  photo blob,
  started timestamp,
  ended timestamp,
  notes text
);
create index hr_staff_department_idx on hr_staff (department_id, status);

create table hr_events (
  event_id integer primary key autoincrement,
  happened_at timestamp not null,
  kind text not null,
  detail text check (detail is null or json_valid(detail))
);

create trigger hr_departments_touch after update of budget, name, head_count, is_active on hr_departments
  for each row
  begin
    update hr_departments set updated_at = current_timestamp where department_id = new.department_id;
  end;

create trigger hr_staff_log_insert after insert on hr_staff
  for each row
  begin
    insert into hr_events (happened_at, kind, detail) values (current_timestamp, 'insert', json_object('staff_id', new.staff_id));
  end;
create trigger hr_staff_log_update after update on hr_staff
  for each row
  begin
    insert into hr_events (happened_at, kind, detail) values (current_timestamp, 'update', json_object('staff_id', new.staff_id));
  end;
create trigger hr_staff_log_delete after delete on hr_staff
  for each row
  begin
    insert into hr_events (happened_at, kind, detail) values (current_timestamp, 'delete', json_object('staff_id', old.staff_id));
  end;

create view hr_active_departments as
  select d.code, d.name, d.budget, (select count(*) from hr_staff s where s.department_id = d.department_id) as staff
  from hr_departments d
  where d.is_active;

-- SQLite has no materialized view, so this is a table that holds the rows of the
-- query when the fixture is made.
create table sales_by_country as
  select c.country, count(distinct o.order_id) as orders,
    sum(od.unit_price * od.quantity * (1 - od.discount)) as sales
  from customers c
    join orders o on o.customer_id = c.customer_id
    join order_details od on od.order_id = o.order_id
  group by c.country;
create unique index sales_by_country_country_idx on sales_by_country (country);

insert into hr_departments (code, name, budget, head_count, is_active, tags, settings, office_street, office_city, office_postal_code, office_country, network, opened_on, review_every) values
  ('SALE', 'Sales', 1250000.00, 4, 1, '["revenue", "field"]', '{"quota": 500000, "regions": ["EMEA", "NA"]}', '1 Market St', 'Seattle', '98101', 'USA', '10.1.0.0/16', '1992-04-01', '6 months'),
  ('ENGR', 'Engineering', 3400000.50, 3, 1, '["product"]', '{"oncall": true}', '2 Kode Way', 'London', 'EC1A 1BB', 'UK', '10.2.0.0/16', '1993-01-15', '1 year'),
  ('FINC', 'Finance', 420000.00, 1, 1, '[]', null, '3 Ledger Rd', 'Tacoma', '98402', 'USA', null, '1992-04-01', '3 months'),
  ('RSCH', 'Research', 0, 0, 0, '["closed"]', '{"closed": "2001-12-31"}', null, null, null, null, null, '1995-06-01', null),
  ('MKTG', 'Marketing — 市場', 780000.00, 1, 1, '["brand", "social media"]', '{"channels": {"tv": 2, "web": 9}}', '4 Rue Lepic', 'Paris', '75018', 'France', '10.4.0.0/16', '1996-09-30', '2 mons 15 days');

update hr_departments set budget = budget * 1.05 where code = 'SALE';

insert into hr_staff (employee_id, department_id, email, status, salary, rating, started, ended, notes) values
  (1, 1, 'nancy.davolio@northwind.test', 'active', 62000, 4.5, '1992-05-01', null, 'Top seller of 1997.'),
  (2, 1, 'andrew.fuller@northwind.test', 'active', 98000, 4.9, '1992-08-14', null, null),
  (3, 1, 'janet.leverling@northwind.test', 'on_leave', 59000, 4.1, '1992-04-01', null, 'Line one.' || char(10) || 'Line two.'),
  (4, 1, 'margaret.peacock@northwind.test', 'active', 61000, null, '1993-05-03', null, null),
  (5, 2, 'steven.buchanan@northwind.test', 'active', 105000, 3.8, '1993-10-17', null, 'Leads the UK office.'),
  (6, 2, 'michael.suyama@northwind.test', 'contractor', null, 3.2, '1993-10-17', '1999-01-01', null),
  (7, 2, 'robert.king@northwind.test', 'active', 88000, 4.0, '1994-01-02', null, null),
  (8, 3, 'laura.callahan@northwind.test', 'active', 54000, 4.4, '1994-03-05', null, 'Inside sales coordinator.'),
  (9, 5, 'anne.dodsworth@northwind.test', 'retired', 0, 5.0, '1994-11-15', '2020-06-30', 'Ünïcödé, émojis 🎉 and a long note that goes on to be wider than most terminals are, so that the wrapped and aligned formats have something to wrap.');
