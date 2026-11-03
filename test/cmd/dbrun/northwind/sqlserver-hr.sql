-- Objects that Northwind lacks, for the describe commands of usql.

create schema hr;

create type hr.email from nvarchar(254) not null;
create type hr.code_list as table (code char(4) primary key);

create sequence hr.ticket_seq as int start with 1000 increment by 10;

create table hr.departments (
  department_id int identity(1, 1) constraint pk_departments primary key,
  code char(4) not null constraint uq_departments_code unique,
  name nvarchar(100) not null,
  budget decimal(14, 2) constraint ck_departments_budget check (budget >= 0),
  head_count smallint not null constraint df_departments_head_count default 0,
  cost_per_head as (budget / nullif(head_count, 0)) persisted,
  is_active bit not null constraint df_departments_is_active default 1,
  settings json,
  office_location geography,
  opened_on date,
  created_at datetimeoffset(3) not null constraint df_departments_created_at default sysdatetimeoffset(),
  updated_at datetime2(3),
  row_version rowversion
);
create index ix_departments_active on hr.departments (name) where is_active = 1;
create index ix_departments_budget on hr.departments (budget desc) include (name);

create table hr.staff (
  staff_id uniqueidentifier not null constraint pk_staff primary key nonclustered constraint df_staff_id default newsequentialid(),
  employee_id int null constraint fk_staff_employees references dbo.Employees (EmployeeID),
  department_id int not null constraint fk_staff_departments references hr.departments (department_id) on delete cascade,
  email hr.email,
  status varchar(12) not null constraint ck_staff_status check (status in ('active', 'on_leave', 'contractor', 'retired')),
  salary money null,
  rating real null,
  photo varbinary(max) null,
  started datetime2(0) null,
  ended datetime2(0) null,
  notes nvarchar(max) null,
  constraint uq_staff_email unique (email)
);
create clustered index ix_staff_department on hr.staff (department_id, status);

create table hr.prices (
  price_id int not null constraint pk_prices primary key,
  product_id int not null constraint fk_prices_products references dbo.Products (ProductID),
  price money not null,
  valid_from datetime2 generated always as row start not null,
  valid_to datetime2 generated always as row end not null,
  period for system_time (valid_from, valid_to)
) with (system_versioning = on (history_table = hr.prices_history));

create partition function pf_event_year (datetime2(0)) as range right for values ('2025-01-01', '2026-01-01');
create partition scheme ps_event_year as partition pf_event_year all to ([primary]);
create table hr.events (
  event_id bigint identity(1, 1) not null,
  happened_at datetime2(0) not null,
  kind varchar(20) not null,
  detail nvarchar(max) null,
  constraint pk_events primary key (event_id, happened_at)
) on ps_event_year (happened_at);

create trigger hr.trg_departments_touch on hr.departments after update as
begin
  set nocount on;
  update d set updated_at = sysdatetime()
  from hr.departments d join inserted i on i.department_id = d.department_id;
end;

create trigger hr.trg_staff_log on hr.staff after insert, update, delete as
begin
  set nocount on;
  insert into hr.events (happened_at, kind, detail)
  select sysdatetime(), case when exists (select 1 from deleted) then 'update' else 'insert' end, convert(nvarchar(36), staff_id)
  from inserted;
end;

create function hr.department_size (@department int) returns int as
begin
  return (select count(*) from hr.staff where department_id = @department);
end;

create function hr.departments_in (@codes hr.code_list readonly) returns table as
return select d.* from hr.departments d join @codes c on c.code = d.code;

create procedure hr.give_raise @department int, @percent decimal(5, 2) as
begin
  set nocount on;
  update hr.staff set salary = salary * (1 + @percent / 100) where department_id = @department;
end;

create view hr.active_departments as
  select d.code, d.name, d.budget, hr.department_size(d.department_id) as staff
  from hr.departments d
  where d.is_active = 1;

create view dbo.sales_by_country with schemabinding as
  select c.Country, count_big(*) as lines, sum(od.UnitPrice * od.Quantity) as gross
  from dbo.Customers c
    join dbo.Orders o on o.CustomerID = c.CustomerID
    join dbo.order_details od on od.OrderID = o.OrderID
  group by c.Country;
create unique clustered index ix_sales_by_country on dbo.sales_by_country (Country);

create synonym dbo.departments for hr.departments;

exec sp_addextendedproperty @name = N'MS_Description', @value = N'the departments of the company',
  @level0type = N'SCHEMA', @level0name = N'hr', @level1type = N'TABLE', @level1name = N'departments';
exec sp_addextendedproperty @name = N'MS_Description', @value = N'budget divided by head count, null with no one',
  @level0type = N'SCHEMA', @level0name = N'hr', @level1type = N'TABLE', @level1name = N'departments',
  @level2type = N'COLUMN', @level2name = N'cost_per_head';

create role northwind_reader;
grant select on schema::hr to northwind_reader;
grant select on schema::dbo to northwind_reader;

insert into hr.departments (code, name, budget, head_count, is_active, settings, office_location, opened_on) values
  ('SALE', N'Sales', 1250000.00, 4, 1, '{"quota": 500000, "regions": ["EMEA", "NA"]}', geography::Point(47.6062, -122.3321, 4326), '1992-04-01'),
  ('ENGR', N'Engineering', 3400000.50, 3, 1, '{"oncall": true}', geography::Point(51.5074, -0.1278, 4326), '1993-01-15'),
  ('FINC', N'Finance', 420000.00, 1, 1, null, null, '1992-04-01'),
  ('RSCH', N'Research', 0, 0, 0, '{"closed": "2001-12-31"}', null, '1995-06-01'),
  ('MKTG', N'Marketing — 市場', 780000.00, 1, 1, '{"channels": {"tv": 2, "web": 9}}', geography::Point(48.8566, 2.3522, 4326), '1996-09-30');

insert into hr.staff (employee_id, department_id, email, status, salary, rating, started, ended, notes) values
  (1, 1, N'nancy.davolio@northwind.test', 'active', 62000, 4.5, '1992-05-01', null, N'Top seller of 1997.'),
  (2, 1, N'andrew.fuller@northwind.test', 'active', 98000, 4.9, '1992-08-14', null, null),
  (3, 1, N'janet.leverling@northwind.test', 'on_leave', 59000, 4.1, '1992-04-01', null, N'Line one.' + char(10) + N'Line two.'),
  (4, 1, N'margaret.peacock@northwind.test', 'active', 61000, null, '1993-05-03', null, null),
  (5, 2, N'steven.buchanan@northwind.test', 'active', 105000, 3.8, '1993-10-17', null, N'Leads the UK office.'),
  (6, 2, N'michael.suyama@northwind.test', 'contractor', null, 3.2, '1993-10-17', '1999-01-01', null),
  (7, 2, N'robert.king@northwind.test', 'active', 88000, 4.0, '1994-01-02', null, null),
  (8, 3, N'laura.callahan@northwind.test', 'active', 54000, 4.4, '1994-03-05', null, N'Inside sales coordinator.'),
  (9, 5, N'anne.dodsworth@northwind.test', 'retired', 0, 5.0, '1994-11-15', '2020-06-30', N'Ünïcödé, émojis 🎉 and a long note that goes on to be wider than most terminals are, so that the wrapped and aligned formats have something to wrap.');

insert into hr.prices (price_id, product_id, price) select ProductID, ProductID, UnitPrice from dbo.Products where ProductID <= 10;
update hr.prices set price = price * 1.10 where price_id <= 5;

update hr.departments set budget = budget * 1.05 where code = 'SALE';
