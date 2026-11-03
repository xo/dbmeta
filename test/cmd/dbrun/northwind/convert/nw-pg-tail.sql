-- the identity columns go on from the largest key that the rows hold
select setval(pg_get_serial_sequence('categories', 'category_id'), max(category_id)) from categories;
select setval(pg_get_serial_sequence('employees', 'employee_id'), max(employee_id)) from employees;
select setval(pg_get_serial_sequence('orders', 'order_id'), max(order_id)) from orders;
select setval(pg_get_serial_sequence('products', 'product_id'), max(product_id)) from products;
select setval(pg_get_serial_sequence('shippers', 'shipper_id'), max(shipper_id)) from shippers;
select setval(pg_get_serial_sequence('suppliers', 'supplier_id'), max(supplier_id)) from suppliers;

-- the stored procedures of the T-SQL script, as functions
create function ten_most_expensive_products()
returns table (ten_most_expensive_products varchar, unit_price money)
language sql stable as $$
  select product_name, unit_price from products order by unit_price desc limit 10
$$;

create function employee_sales_by_country(beginning_date timestamp, ending_date timestamp)
returns table (country varchar, last_name varchar, first_name varchar, shipped_date timestamp, order_id integer, sale_amount money)
language sql stable as $$
  select e.country, e.last_name, e.first_name, o.shipped_date, o.order_id, s.subtotal
  from employees e
    join orders o on e.employee_id = o.employee_id
    join order_subtotals s on o.order_id = s.order_id
  where o.shipped_date between beginning_date and ending_date
$$;

create function sales_by_year(beginning_date timestamp, ending_date timestamp)
returns table (shipped_date timestamp, order_id integer, subtotal money, year text)
language sql stable as $$
  select o.shipped_date, o.order_id, s.subtotal, to_char(o.shipped_date, 'YYYY')
  from orders o join order_subtotals s on o.order_id = s.order_id
  where o.shipped_date between beginning_date and ending_date
$$;

create function cust_orders_detail(order_id integer)
returns table (product_name varchar, unit_price numeric, quantity smallint, discount integer, extended_price numeric)
language sql stable as $$
  select p.product_name, round(od.unit_price::numeric, 2), od.quantity,
    (od.discount * 100)::integer,
    round((od.quantity * (1 - od.discount) * od.unit_price::numeric)::numeric, 2)
  from products p join order_details od on od.product_id = p.product_id
  where od.order_id = cust_orders_detail.order_id
$$;

create function cust_orders_orders(customer_id char(5))
returns table (order_id integer, order_date timestamp, required_date timestamp, shipped_date timestamp)
language sql stable as $$
  select o.order_id, o.order_date, o.required_date, o.shipped_date
  from orders o
  where o.customer_id = cust_orders_orders.customer_id
  order by o.order_id
$$;

create function cust_order_hist(customer_id char(5))
returns table (product_name varchar, total bigint)
language sql stable as $$
  select p.product_name, sum(od.quantity)
  from products p, order_details od, orders o, customers c
  where c.customer_id = cust_order_hist.customer_id
    and c.customer_id = o.customer_id and o.order_id = od.order_id and od.product_id = p.product_id
  group by p.product_name
$$;

create function sales_by_category(category_name varchar, ord_year varchar default '1998')
returns table (product_name varchar, total_purchase numeric)
language plpgsql stable as $$
begin
  if ord_year not in ('1996', '1997', '1998') then
    ord_year := '1998';
  end if;
  return query
    select p.product_name,
      round(sum((od.quantity * (1 - od.discount) * od.unit_price::numeric)::numeric(14, 2)), 0)
    from order_details od, orders o, products p, categories c
    where od.order_id = o.order_id
      and od.product_id = p.product_id
      and p.category_id = c.category_id
      and c.category_name = sales_by_category.category_name
      and to_char(o.order_date, 'YYYY') = ord_year
    group by p.product_name
    order by p.product_name;
end
$$
