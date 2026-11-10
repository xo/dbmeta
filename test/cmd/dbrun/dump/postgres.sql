-- Fixture for dbrun fixture dump on PostgreSQL. See D291.
-- The database is called dump and it has the schemas core and extra.

-- Section: schemas
CREATE SCHEMA core;
CREATE SCHEMA extra;
CREATE SCHEMA "Mixed Case";
CREATE SCHEMA "same";

-- Section: types
CREATE TYPE core.mood AS ENUM ('sad', 'happy', 'angry');
-- The new value goes between two old ones, so the order of the enum is not the order of creation.
ALTER TYPE core.mood ADD VALUE 'ok' BEFORE 'happy';
ALTER TYPE core.mood ADD VALUE 'calm' AFTER 'sad';
-- A domain over an enum, with a default and a check.
CREATE DOMAIN core.mood_d AS core.mood DEFAULT 'ok' NOT NULL CHECK (VALUE <> 'angry');
-- A range type and its multirange. The multirange has a name that is not the default.
CREATE FUNCTION extra.float_gap(a float8, b float8) RETURNS float8
    LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT a - b $$;
CREATE TYPE extra.floatrange AS RANGE (
    subtype = float8,
    subtype_diff = extra.float_gap,
    multirange_type_name = extra.floatmulti
);
CREATE TYPE core.pair AS (left_side text, right_side int);
CREATE TYPE extra.pair AS (a int, b int);
-- A type with the same name as a schema, a table and a column (see names below).
CREATE TYPE extra."same" AS ENUM ('one', 'two');

-- Section: routines
-- One user function that a check, a default, a generated column and an expression index all call.
CREATE FUNCTION core.norm(t text) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT
    AS $$ SELECT lower(btrim(t)) $$;

-- Overloaded functions.
CREATE FUNCTION core.add(a int, b int) RETURNS int
    LANGUAGE sql IMMUTABLE AS $$ SELECT a + b $$;
CREATE FUNCTION core.add(a numeric, b numeric) RETURNS numeric
    LANGUAGE sql IMMUTABLE AS $$ SELECT a + b $$;
CREATE FUNCTION core.add(a text, b text) RETURNS text
    LANGUAGE sql IMMUTABLE AS $$ SELECT a || b $$;

-- OUT and INOUT parameters.
CREATE FUNCTION core.split(IN s text, OUT head text, INOUT n int, OUT tail text)
    LANGUAGE plpgsql IMMUTABLE AS $$
BEGIN
    head := left(s, n);
    tail := substr(s, n + 1);
    n := length(s);
END
$$;

-- A type in the signature, both an enum and a composite type.
CREATE FUNCTION core.describe(m core.mood, p core.pair) RETURNS text
    LANGUAGE sql IMMUTABLE AS $$ SELECT m::text || ':' || p.left_side || p.right_side $$;
CREATE FUNCTION extra.describe(m core.mood, p extra.pair) RETURNS text
    LANGUAGE sql IMMUTABLE AS $$ SELECT m::text || ':' || p.a || p.b $$;

-- Default arguments.
CREATE FUNCTION core.greet(name text DEFAULT 'world', punct text DEFAULT '!') RETURNS text
    LANGUAGE sql IMMUTABLE AS $$ SELECT 'hello ' || name || punct $$;

-- Volatility: immutable, stable and volatile.
CREATE FUNCTION core.v_immutable(x int) RETURNS int LANGUAGE sql IMMUTABLE AS $$ SELECT x $$;
CREATE FUNCTION core.v_stable(x int) RETURNS int LANGUAGE sql STABLE AS $$ SELECT x $$;
CREATE FUNCTION core.v_volatile(x int) RETURNS int LANGUAGE sql VOLATILE AS $$ SELECT x $$;

-- SECURITY DEFINER with a fixed search path, and PARALLEL SAFE with COST and ROWS.
CREATE FUNCTION core.definer() RETURNS text
    LANGUAGE sql SECURITY DEFINER SET search_path = core, pg_temp
    AS $$ SELECT current_user::text $$;
CREATE FUNCTION core.series(n int) RETURNS SETOF int
    LANGUAGE sql IMMUTABLE PARALLEL SAFE COST 10 ROWS 100
    AS $$ SELECT generate_series(1, n) $$;

-- RETURNS TABLE and SETOF.
CREATE FUNCTION core.pairs(n int) RETURNS TABLE (i int, label text)
    LANGUAGE sql IMMUTABLE
    AS $$ SELECT g, 'n' || g FROM generate_series(1, n) AS g $$;
CREATE FUNCTION core.moods() RETURNS SETOF core.mood
    LANGUAGE sql IMMUTABLE AS $$ SELECT unnest(enum_range(NULL::core.mood)) $$;

-- A SQL body written with BEGIN ATOMIC. The statements inside it end with semicolons.
CREATE FUNCTION core.twice(x int) RETURNS int
    LANGUAGE sql IMMUTABLE
    BEGIN ATOMIC
        SELECT x * 2;
    END;

-- A procedure with INOUT.
CREATE PROCEDURE core.bump(INOUT n int)
    LANGUAGE plpgsql AS $$
BEGIN
    n := n + 1;
END
$$;

-- An aggregate and an operator that use the function above.
CREATE AGGREGATE core.sum_twice(int) (
    sfunc = int4pl,
    stype = int,
    initcond = '0'
);

-- Section: tables and indexes
CREATE SEQUENCE core.people_seq START 100 INCREMENT 5;

-- core.people has a foreign key to itself. The function core.norm is called by
-- a check, a default, a generated column and an expression index.
CREATE TABLE core.people (
    id int PRIMARY KEY DEFAULT nextval('core.people_seq'),
    name text NOT NULL CHECK (core.norm(name) <> ''),
    boss int REFERENCES core.people (id) DEFERRABLE INITIALLY DEFERRED,
    mood core.mood_d,
    slug text DEFAULT core.norm('Default Slug'),
    name_norm text GENERATED ALWAYS AS (core.norm(name)) STORED,
    born date,
    score numeric(8, 2),
    active boolean NOT NULL DEFAULT true,
    bio text,
    span extra.floatrange,
    scratch int
);
CREATE INDEX people_norm_idx ON core.people (core.norm(name));
CREATE UNIQUE INDEX people_slug_idx ON core.people (slug, id) INCLUDE (name) WHERE active;
-- A column that is dropped after the table is created leaves a gap in the column numbers.
ALTER TABLE core.people DROP COLUMN scratch;
COMMENT ON TABLE core.people IS 'People of the core schema, with a quote '' in the comment';
COMMENT ON COLUMN core.people.name IS 'The name, in any script';

CREATE TABLE extra.people (
    id int PRIMARY KEY,
    name text NOT NULL,
    mood core.mood,
    born date,
    score numeric(8, 2),
    active boolean NOT NULL DEFAULT true,
    bio text,
    span extra.floatrange,
    spans extra.floatmulti
);

INSERT INTO core.people (id, name, boss, mood, slug, born, score, active, bio, span) VALUES
    (1, 'Ada Lovelace', NULL, 'happy', 'ada', '1815-12-10', 99.50, true, 'First programmer', '[1.5,2.5)'),
    (2, 'Zoë Müller', 1, 'calm', 'zoe', '1990-01-02', 12.25, true, NULL, NULL),
    (3, '日本語 テスト', 1, 'sad', NULL, NULL, NULL, false, 'Emoji 😀 and tab', 'empty'),
    (4, 'O''Brien "Quoted"', 2, 'ok', 'obrien', '2000-02-29', -3.00, true, E'line one\nline two', '(,3]');
INSERT INTO extra.people (id, name, mood, born, score, active, bio, span, spans) VALUES
    (1, 'Grace Hopper', 'happy', '1906-12-09', 88.00, true, 'Admiral', '[0,1]', '{[0,1],[5,6]}'),
    (2, 'Åsa Öberg', NULL, NULL, NULL, true, NULL, NULL, NULL),
    (3, 'Ünïcödé ☃', 'sad', '1999-12-31', 0.01, false, 'Snowman ☃', '(1,2)', '{}');

-- A partitioned table with a default partition and a subpartition.
CREATE TABLE core.events (
    id int NOT NULL,
    at date NOT NULL,
    kind text NOT NULL,
    payload text
) PARTITION BY RANGE (at);
CREATE TABLE core.events_2024 PARTITION OF core.events
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01')
    PARTITION BY LIST (kind);
CREATE TABLE core.events_2024_a PARTITION OF core.events_2024 FOR VALUES IN ('a');
CREATE TABLE core.events_2024_other PARTITION OF core.events_2024 DEFAULT;
CREATE TABLE core.events_default PARTITION OF core.events DEFAULT;
-- An index on the partitioned parent. PostgreSQL makes an index on each partition.
CREATE INDEX events_kind_idx ON core.events (kind, at);
INSERT INTO core.events VALUES
    (1, '2024-03-01', 'a', 'in the subpartition'),
    (2, '2024-04-01', 'b', NULL),
    (3, '2020-01-01', 'a', 'in the default partition');

-- EXCLUDE USING gist.
CREATE TABLE core.booking (
    id int PRIMARY KEY,
    during tstzrange NOT NULL,
    EXCLUDE USING gist (during WITH &&)
);
INSERT INTO core.booking VALUES
    (1, '[2024-01-01 00:00+00,2024-01-02 00:00+00)'),
    (2, '[2024-01-02 00:00+00,2024-01-03 00:00+00)');

-- Section: dependencies
-- Foreign keys that refer to each other. The second key is added after both tables exist.
CREATE TABLE core.parent_a (
    id int PRIMARY KEY,
    b_id int
);
CREATE TABLE core.parent_b (
    id int PRIMARY KEY,
    a_id int REFERENCES core.parent_a (id)
);
ALTER TABLE core.parent_a ADD CONSTRAINT parent_a_b_fk FOREIGN KEY (b_id) REFERENCES core.parent_b (id) DEFERRABLE INITIALLY DEFERRED;
INSERT INTO core.parent_a VALUES (1, NULL);
INSERT INTO core.parent_b VALUES (1, 1);
UPDATE core.parent_a SET b_id = 1 WHERE id = 1;
-- A foreign key from the schema extra to the schema core.
ALTER TABLE extra.people ADD CONSTRAINT people_core_fk FOREIGN KEY (id) REFERENCES core.people (id) NOT VALID;

-- A view that calls a function that reads a table.
CREATE FUNCTION core.people_count() RETURNS bigint
    LANGUAGE sql STABLE AS $$ SELECT count(*) FROM core.people $$;
CREATE VIEW core.people_total AS SELECT core.people_count() AS total;
CREATE VIEW extra.people_names AS
    SELECT p.id, p.name, core.describe(p.mood::core.mood, ROW('x', p.id)::core.pair) AS described
    FROM extra.people AS p;
CREATE MATERIALIZED VIEW extra.people_by_mood AS
    SELECT mood, count(*) AS n FROM core.people GROUP BY mood;

-- A view and a function that depend on each other in a loop. PostgreSQL records
-- only one direction, because a plpgsql body is not checked when it is created.
CREATE FUNCTION core.loop_f(n int) RETURNS bigint
    LANGUAGE plpgsql STABLE AS $$
BEGIN
    RETURN (SELECT count(*) FROM core.loop_v) + n;
END
$$;
CREATE VIEW core.loop_v AS SELECT id, core.loop_f(0) AS total FROM core.people WHERE id < 3;

-- Section: triggers
CREATE TABLE core.audit (
    at timestamptz DEFAULT now(),
    what text NOT NULL
);

CREATE FUNCTION core.log_trigger() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO core.audit (what) VALUES (TG_NAME || ' ' || TG_OP || ' ' || TG_LEVEL);
    IF TG_LEVEL = 'ROW' THEN
        RETURN COALESCE(NEW, OLD);
    END IF;
    RETURN NULL;
END
$$;

-- A statement trigger.
CREATE TRIGGER people_stmt AFTER INSERT OR DELETE ON core.people
    FOR EACH STATEMENT EXECUTE FUNCTION core.log_trigger();
-- A WHEN condition.
CREATE TRIGGER people_when BEFORE UPDATE ON core.people
    FOR EACH ROW WHEN (OLD.name IS DISTINCT FROM NEW.name)
    EXECUTE FUNCTION core.log_trigger();
-- UPDATE OF a column.
CREATE TRIGGER people_of AFTER UPDATE OF score, bio ON core.people
    FOR EACH ROW EXECUTE FUNCTION core.log_trigger();
-- A deferrable constraint trigger.
CREATE CONSTRAINT TRIGGER people_deferred AFTER INSERT ON core.people
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION core.log_trigger();
-- Transition tables. A trigger with them cannot name columns.
CREATE FUNCTION core.log_transition() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO core.audit (what) SELECT 'moved ' || count(*) FROM moved_rows;
    RETURN NULL;
END
$$;
CREATE TRIGGER people_moved AFTER INSERT ON extra.people
    REFERENCING NEW TABLE AS moved_rows
    FOR EACH STATEMENT EXECUTE FUNCTION core.log_transition();
-- A trigger on a partitioned table. PostgreSQL makes one on each partition.
CREATE TRIGGER events_row AFTER INSERT ON core.events
    FOR EACH ROW EXECUTE FUNCTION core.log_trigger();
-- INSTEAD OF on a view.
CREATE VIEW core.people_editable AS SELECT id, name FROM core.people;
CREATE FUNCTION core.edit_people() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    UPDATE core.people SET name = NEW.name WHERE id = OLD.id;
    RETURN NEW;
END
$$;
CREATE TRIGGER people_instead INSTEAD OF UPDATE ON core.people_editable
    FOR EACH ROW EXECUTE FUNCTION core.edit_people();
-- A disabled trigger.
CREATE TRIGGER people_off BEFORE DELETE ON core.people
    FOR EACH ROW EXECUTE FUNCTION core.log_trigger();
ALTER TABLE core.people DISABLE TRIGGER people_off;
-- An event trigger.
CREATE FUNCTION core.on_ddl() RETURNS event_trigger
    LANGUAGE plpgsql AS $$
BEGIN
    NULL;
END
$$;
CREATE EVENT TRIGGER dump_ddl ON ddl_command_end
    WHEN TAG IN ('CREATE TABLE', 'ALTER TABLE')
    EXECUTE FUNCTION core.on_ddl();

-- Section: names
-- Mixed case, a space, a reserved word, Unicode and an embedded quote.
CREATE TABLE "Mixed Case"."select" (
    "Id" int PRIMARY KEY,
    "order" text,
    "say ""hi""" text,
    "naïve ☃ 日本" text,
    "with space" int DEFAULT 7
);
CREATE INDEX "Mixed ""Index""" ON "Mixed Case"."select" ("order", "with space");
CREATE VIEW "Mixed Case"."User View" AS SELECT "Id" AS "from", "say ""hi""" AS "to" FROM "Mixed Case"."select";
CREATE FUNCTION "Mixed Case"."Do It"("Input Value" int) RETURNS int
    LANGUAGE sql IMMUTABLE AS $$ SELECT "Input Value" + 1 $$;
INSERT INTO "Mixed Case"."select" VALUES
    (1, 'group', 'He said "hi"', 'naïve', 1),
    (2, NULL, NULL, '日本語', NULL),
    (3, 'where', 'it''s', '☃ 😀', 3);

-- One name used at once for a schema, a table, a column and a type.
-- A table and a type share one namespace in a schema, so the type is in extra.
CREATE TABLE "same"."same" (
    "same" extra."same" NOT NULL,
    id int
);
INSERT INTO "same"."same" VALUES ('one', 1), ('two', NULL);

-- Row level security: a policy that restricts, a policy for a role, and the switch.
CREATE POLICY people_visible ON core.people FOR SELECT USING (id > 0);
CREATE POLICY people_edit ON core.people AS RESTRICTIVE FOR UPDATE TO PUBLIC USING (true) WITH CHECK (id > 0);
ALTER TABLE core.people ENABLE ROW LEVEL SECURITY;

-- An ordered set aggregate.
CREATE AGGREGATE core.pick_disc(float8 ORDER BY anyelement) (
    SFUNC = ordered_set_transition,
    STYPE = internal,
    FINALFUNC = percentile_disc_final,
    FINALFUNC_EXTRA
);

-- A moving aggregate, with the options of a parallel plan and of the size of the state.
CREATE AGGREGATE core.moving_total(int) (
    SFUNC = int4pl,
    STYPE = int,
    SSPACE = 16,
    INITCOND = '0',
    MSFUNC = int4pl,
    MINVFUNC = int4mi,
    MSTYPE = int,
    MINITCOND = '0',
    PARALLEL = SAFE
);

-- Intervals, with months, days and a time apart, and a negative one.
CREATE TABLE core.intervals (
    id int PRIMARY KEY,
    every interval,
    short interval day to second(3),
    span interval year to month
);
INSERT INTO core.intervals VALUES
    (1, '1 year 2 months 3 days 04:05:06.789', '2 days 03:04:05.678', '3 years 4 months'),
    (2, '-5 days -01:00:00', '-00:00:01.5', '-1 year -2 months'),
    (3, NULL, NULL, NULL);

-- Roles and grants: a role that cannot log in, one that is a member of it, and grants on a table, on columns
-- and on a sequence. A role belongs to the cluster, so the file makes it when it is not there.
DO $$
BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'dump_reader') THEN
    CREATE ROLE dump_reader NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'dump_member') THEN
    CREATE ROLE dump_member NOLOGIN CREATEDB;
  END IF;
END
$$;
GRANT dump_reader TO dump_member;
GRANT SELECT ON core.people TO dump_reader;
GRANT SELECT, INSERT, UPDATE ON core.audit TO dump_reader WITH GRANT OPTION;
GRANT SELECT (id), UPDATE (b_id) ON core.parent_a TO dump_member;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA core TO dump_reader;
GRANT SELECT ON core.people_total TO PUBLIC;
GRANT USAGE ON SCHEMA core TO dump_reader;
GRANT EXECUTE ON FUNCTION core.add(integer, integer) TO dump_reader;
GRANT USAGE ON TYPE core.mood TO dump_reader;
