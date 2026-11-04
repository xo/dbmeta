
-- SQLite has no stored function that returns a table. The one stored
-- procedure of the T-SQL script that takes no argument is a view.
create view ten_most_expensive_products as
  select product_name as ten_most_expensive_products, unit_price
  from products order by unit_price desc limit 10;
