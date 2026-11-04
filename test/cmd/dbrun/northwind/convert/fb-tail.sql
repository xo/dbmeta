
-- the stored procedures of the T-SQL script, as selectable procedures
create procedure ten_most_expensive_products
returns (ten_most_expensive_products varchar(40), unit_price decimal(18,4)) as
begin
  for select first 10 product_name, unit_price from products order by unit_price desc
    into :ten_most_expensive_products, :unit_price
  do suspend;
end;

create procedure employee_sales_by_country(beginning_date timestamp, ending_date timestamp)
returns (country varchar(15), last_name varchar(20), first_name varchar(10), shipped_date timestamp,
  order_id integer, sale_amount numeric(18,2)) as
begin
  for select e.country, e.last_name, e.first_name, o.shipped_date, o.order_id, s.subtotal
    from employees e
      join orders o on e.employee_id = o.employee_id
      join order_subtotals s on o.order_id = s.order_id
    where o.shipped_date between :beginning_date and :ending_date
    into :country, :last_name, :first_name, :shipped_date, :order_id, :sale_amount
  do suspend;
end;

create procedure sales_by_year(beginning_date timestamp, ending_date timestamp)
returns (shipped_date timestamp, order_id integer, subtotal numeric(18,2), shipped_year varchar(4)) as
begin
  for select o.shipped_date, o.order_id, s.subtotal, cast(extract(year from o.shipped_date) as varchar(4))
    from orders o join order_subtotals s on o.order_id = s.order_id
    where o.shipped_date between :beginning_date and :ending_date
    into :shipped_date, :order_id, :subtotal, :shipped_year
  do suspend;
end;

create procedure cust_orders_detail(order_id_in integer)
returns (product_name varchar(40), unit_price numeric(18,2), quantity smallint, discount integer,
  extended_price numeric(18,2)) as
begin
  for select p.product_name, round(od.unit_price, 2), od.quantity, cast(od.discount * 100 as integer),
      round(od.quantity * (1 - od.discount) * od.unit_price, 2)
    from products p join order_details od on od.product_id = p.product_id
    where od.order_id = :order_id_in
    into :product_name, :unit_price, :quantity, :discount, :extended_price
  do suspend;
end;

create procedure cust_orders_orders(customer_id_in char(5))
returns (order_id integer, order_date timestamp, required_date timestamp, shipped_date timestamp) as
begin
  for select o.order_id, o.order_date, o.required_date, o.shipped_date
    from orders o where o.customer_id = :customer_id_in order by o.order_id
    into :order_id, :order_date, :required_date, :shipped_date
  do suspend;
end;

create procedure cust_order_hist(customer_id_in char(5))
returns (product_name varchar(40), total bigint) as
begin
  for select p.product_name, sum(od.quantity)
    from products p, order_details od, orders o, customers c
    where c.customer_id = :customer_id_in
      and c.customer_id = o.customer_id and o.order_id = od.order_id and od.product_id = p.product_id
    group by p.product_name
    into :product_name, :total
  do suspend;
end;

create procedure sales_by_category_for_year(category_name_in varchar(15), ord_year varchar(4))
returns (product_name varchar(40), total_purchase numeric(18,0)) as
begin
  if (ord_year not in ('1996', '1997', '1998')) then ord_year = '1998';
  for select p.product_name, round(sum(round(od.quantity * (1 - od.discount) * od.unit_price, 2)), 0)
    from order_details od, orders o, products p, categories c
    where od.order_id = o.order_id
      and od.product_id = p.product_id
      and p.category_id = c.category_id
      and c.category_name = :category_name_in
      and cast(extract(year from o.order_date) as varchar(4)) = :ord_year
    group by p.product_name
    order by p.product_name
    into :product_name, :total_purchase
  do suspend;
end;
