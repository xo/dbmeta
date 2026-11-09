-- What MariaDB has and MySQL lacks.

create sequence hr.ticket_seq start with 1000 increment by 10;

-- MariaDB has no index on an expression, so a virtual column holds it.
alter table hr.departments
  add column name_lower varchar(100) generated always as (lower(name)) virtual,
  add index departments_lower_name_idx (name_lower);

create aggregate function hr.concat_lines(line text) returns text
begin
  declare all_lines text default '';
  declare continue handler for not found return all_lines;
  loop
    fetch group next row;
    set all_lines = concat(all_lines, line);
  end loop;
end;
