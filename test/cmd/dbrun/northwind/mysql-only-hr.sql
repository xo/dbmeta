-- What MySQL has and MariaDB lacks.

create index departments_lower_name_idx on hr.departments ((lower(name)));
create index departments_regions_idx on hr.departments ((cast(settings -> '$.regions' as char(10) array)));
