
-- the stored procedures of the T-SQL script. MySQL has no function that
-- returns a table, so each one is a procedure that returns a result set
create procedure ten_most_expensive_products()
  select product_name as ten_most_expensive_products, unit_price
  from products order by unit_price desc limit 10;

create procedure employee_sales_by_country(in beginning_date datetime, in ending_date datetime)
  select e.country, e.last_name, e.first_name, o.shipped_date, o.order_id, s.subtotal as sale_amount
  from employees e
    join orders o on e.employee_id = o.employee_id
    join order_subtotals s on o.order_id = s.order_id
  where o.shipped_date between beginning_date and ending_date;

create procedure sales_by_year(in beginning_date datetime, in ending_date datetime)
  select o.shipped_date, o.order_id, s.subtotal, date_format(o.shipped_date, '%Y') as year
  from orders o join order_subtotals s on o.order_id = s.order_id
  where o.shipped_date between beginning_date and ending_date;

create procedure cust_orders_detail(in order_id integer)
  select p.product_name, round(od.unit_price, 2) as unit_price, od.quantity,
    cast(od.discount * 100 as signed) as discount,
    round(od.quantity * (1 - od.discount) * od.unit_price, 2) as extended_price
  from products p join order_details od on od.product_id = p.product_id
  where od.order_id = order_id;

create procedure cust_orders_orders(in customer_id char(5))
  select o.order_id, o.order_date, o.required_date, o.shipped_date
  from orders o
  where o.customer_id = customer_id
  order by o.order_id;

create procedure cust_order_hist(in customer_id char(5))
  select p.product_name, sum(od.quantity) as total
  from products p, order_details od, orders o, customers c
  where c.customer_id = customer_id
    and c.customer_id = o.customer_id and o.order_id = od.order_id and od.product_id = p.product_id
  group by p.product_name;

create procedure sales_by_category(in category_name varchar(15), in ord_year varchar(4))
  select p.product_name, round(sum(round(od.quantity * (1 - od.discount) * od.unit_price, 2)), 0) as total_purchase
  from order_details od, orders o, products p, categories c
  where od.order_id = o.order_id
    and od.product_id = p.product_id
    and p.category_id = c.category_id
    and c.category_name = category_name
    and date_format(o.order_date, '%Y') = if(ord_year in ('1996', '1997', '1998'), ord_year, '1998')
  group by p.product_name
  order by p.product_name;
