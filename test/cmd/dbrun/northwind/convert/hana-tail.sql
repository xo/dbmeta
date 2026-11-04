
-- the stored procedures of the T-SQL script, as table functions
create function ten_most_expensive_products()
returns table (ten_most_expensive_products nvarchar(40), unit_price decimal(19,4)) as
begin
  return select top 10 product_name as ten_most_expensive_products, unit_price from products order by unit_price desc;
end;

create function employee_sales_by_country(beginning_date timestamp, ending_date timestamp)
returns table (country nvarchar(15), last_name nvarchar(20), first_name nvarchar(10), shipped_date timestamp,
  order_id integer, sale_amount decimal(19,2)) as
begin
  return select e.country, e.last_name, e.first_name, o.shipped_date, o.order_id, s.subtotal as sale_amount
    from employees e
      join orders o on e.employee_id = o.employee_id
      join order_subtotals s on o.order_id = s.order_id
    where o.shipped_date between :beginning_date and :ending_date;
end;

create function sales_by_year(beginning_date timestamp, ending_date timestamp)
returns table (shipped_date timestamp, order_id integer, subtotal decimal(19,2), shipped_year nvarchar(4)) as
begin
  return select o.shipped_date, o.order_id, s.subtotal, to_nvarchar(o.shipped_date, 'YYYY') as shipped_year
    from orders o join order_subtotals s on o.order_id = s.order_id
    where o.shipped_date between :beginning_date and :ending_date;
end;

create function cust_orders_detail(order_id_in integer)
returns table (product_name nvarchar(40), unit_price decimal(19,2), quantity smallint, discount integer,
  extended_price decimal(19,2)) as
begin
  return select p.product_name, round(od.unit_price, 2) as unit_price, od.quantity, cast(od.discount * 100 as integer) as discount,
      round(od.quantity * (1 - od.discount) * od.unit_price, 2) as extended_price
    from products p join order_details od on od.product_id = p.product_id
    where od.order_id = :order_id_in;
end;

create function cust_orders_orders(customer_id_in nvarchar(5))
returns table (order_id integer, order_date timestamp, required_date timestamp, shipped_date timestamp) as
begin
  return select o.order_id, o.order_date, o.required_date, o.shipped_date
    from orders o where o.customer_id = :customer_id_in order by o.order_id;
end;

create function cust_order_hist(customer_id_in nvarchar(5))
returns table (product_name nvarchar(40), total bigint) as
begin
  return select p.product_name, sum(od.quantity) as total
    from products p, order_details od, orders o, customers c
    where c.customer_id = :customer_id_in
      and c.customer_id = o.customer_id and o.order_id = od.order_id and od.product_id = p.product_id
    group by p.product_name;
end;

create function sales_by_category_for_year(category_name_in nvarchar(15), ord_year nvarchar(4))
returns table (product_name nvarchar(40), total_purchase decimal(19,0)) as
begin
  return select p.product_name, round(sum(round(od.quantity * (1 - od.discount) * od.unit_price, 2)), 0) as total_purchase
    from order_details od, orders o, products p, categories c
    where od.order_id = o.order_id
      and od.product_id = p.product_id
      and p.category_id = c.category_id
      and c.category_name = :category_name_in
      and to_nvarchar(o.order_date, 'YYYY') = case when :ord_year in ('1996', '1997', '1998') then :ord_year else '1998' end
    group by p.product_name
    order by p.product_name;
end;
