create or replace function ten_most_expensive_products return sys_refcursor as
  c sys_refcursor;
begin
  open c for
    select product_name as ten_most_expensive_products, unit_price
    from products order by unit_price desc fetch first 10 rows only;
  return c;
end;
create or replace function employee_sales_by_country(beginning_date timestamp, ending_date timestamp) return sys_refcursor as
  c sys_refcursor;
begin
  open c for
    select e.country, e.last_name, e.first_name, o.shipped_date, o.order_id, s.subtotal as sale_amount
    from employees e
      join orders o on e.employee_id = o.employee_id
      join order_subtotals s on o.order_id = s.order_id
    where o.shipped_date between beginning_date and ending_date;
  return c;
end;
create or replace function sales_by_year(beginning_date timestamp, ending_date timestamp) return sys_refcursor as
  c sys_refcursor;
begin
  open c for
    select o.shipped_date, o.order_id, s.subtotal, to_char(o.shipped_date, 'YYYY') as year
    from orders o join order_subtotals s on o.order_id = s.order_id
    where o.shipped_date between beginning_date and ending_date;
  return c;
end;
create or replace function cust_orders_detail(order_id integer) return sys_refcursor as
  c sys_refcursor;
begin
  open c for
    select p.product_name, round(od.unit_price, 2) as unit_price, od.quantity,
      od.discount * 100 as discount,
      round(od.quantity * (1 - od.discount) * od.unit_price, 2) as extended_price
    from products p join order_details od on od.product_id = p.product_id
    where od.order_id = cust_orders_detail.order_id;
  return c;
end;
create or replace function cust_orders_orders(customer_id char) return sys_refcursor as
  c sys_refcursor;
begin
  open c for
    select o.order_id, o.order_date, o.required_date, o.shipped_date
    from orders o
    where o.customer_id = cust_orders_orders.customer_id
    order by o.order_id;
  return c;
end;
create or replace function cust_order_hist(customer_id char) return sys_refcursor as
  c sys_refcursor;
begin
  open c for
    select p.product_name, sum(od.quantity) as total
    from products p, order_details od, orders o, customers c2
    where c2.customer_id = cust_order_hist.customer_id
      and c2.customer_id = o.customer_id and o.order_id = od.order_id and od.product_id = p.product_id
    group by p.product_name;
  return c;
end;
create or replace function sales_by_category_for_year(category_name varchar2, ord_year varchar2 default '1998') return sys_refcursor as
  c sys_refcursor;
  y varchar2(4) := ord_year;
begin
  if y not in ('1996', '1997', '1998') then
    y := '1998';
  end if;
  open c for
    select p.product_name,
      round(sum(od.quantity * (1 - od.discount) * od.unit_price), 0) as total_purchase
    from order_details od, orders o, products p, categories cat
    where od.order_id = o.order_id
      and od.product_id = p.product_id
      and p.category_id = cat.category_id
      and cat.category_name = sales_by_category_for_year.category_name
      and to_char(o.order_date, 'YYYY') = y
    group by p.product_name
    order by p.product_name;
  return c;
end;
