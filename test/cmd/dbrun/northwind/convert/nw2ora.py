"""nw2ora converts the PostgreSQL Northwind script to Oracle.

usage: python3 -I nw2ora.py postgres.sql out.sql ora-tail.sql

It reads the tables, the indexes, the rows, the foreign keys and the views of
the PostgreSQL file, and leaves its functions out, because ora-tail.sql holds
the same procedures written in PL/SQL. Each statement of the output ends with a
semicolon, which is how usql splits an Oracle file. A PL/SQL block ends with its
own END and a semicolon, and a slash line is not written.
"""
import re
import sys


def split(src):
    """Split at each semicolon outside a quote, a comment or a $$ string."""
    out, buf, i, n = [], [], 0, len(src)
    while i < n:
        c = src[i]
        if src.startswith("--", i):
            j = src.find("\n", i)
            j = n if j < 0 else j
            buf.append(src[i:j])
            i = j
        elif c == "'":
            j = i + 1
            while j < n:
                if src[j] == "'":
                    if j + 1 < n and src[j + 1] == "'":
                        j += 2
                        continue
                    break
                j += 1
            buf.append(src[i:j + 1])
            i = j + 1
        elif c == "$" and re.match(r"\$\w*\$", src[i:]):
            tag = re.match(r"\$\w*\$", src[i:]).group(0)
            j = src.find(tag, i + len(tag))
            buf.append(src[i:j + len(tag)])
            i = j + len(tag)
        elif c == ";":
            out.append("".join(buf).strip())
            buf = []
            i += 1
        else:
            buf.append(c)
            i += 1
    rest = "".join(buf).strip()
    if rest and not all(l.strip().startswith("--") or not l.strip() for l in rest.split("\n")):
        out.append(rest)
    return [s for s in out if s]


def strip_comments(s):
    """Remove the whole line comments that sit outside a quote."""
    return "\n".join(l for l in s.split("\n") if not l.strip().startswith("--"))


TYPES = [
    (r"\bvarchar\((\d+)\)", r"varchar2(\1)"),
    (r"\bbytea\b", "blob"),
    (r"\btext\b", "clob"),
    (r"\bmoney\b", "number(19,4)"),
    (r"\bsmallint\b", "number(5)"),
    (r"\breal\b", "binary_float"),
]


def table(s):
    s = s.replace("0::money", "0")
    for a, b in TYPES:
        s = re.sub(a, b, s)
    # Oracle takes no non deterministic function in a check.
    s = re.sub(r",\s*constraint\s+ck_birthdate\s+check\s*\(birth_date < now\(\)\)", "", s)
    s = s.replace("0::money", "0")
    # Oracle writes DEFAULT before NULL and NOT NULL.
    s = re.sub(r"\b(not null|null) default \((\d+)\)", r"default \2 \1", s)
    return s


def literal_dates(s):
    return re.sub(r"'(\d{4}-\d{2}-\d{2})'", r"date '\1'", s)


def values(s):
    """Return the top level items of the first VALUES list of an insert."""
    i = s.upper().index("VALUES")
    j = s.index("(", i)
    items, depth, k, start = [], 0, j + 1, j + 1
    while k < len(s):
        c = s[k]
        if c == "'":
            k += 1
            while True:
                if s[k] == "'":
                    if s[k + 1] == "'":
                        k += 2
                        continue
                    break
                k += 1
        elif c == "(":
            depth += 1
        elif c == ")":
            if depth == 0:
                items.append(s[start:k])
                return items, i, j, k
            depth -= 1
        elif c == "," and depth == 0:
            items.append(s[start:k])
            start = k + 1
        k += 1
    raise ValueError("no end of VALUES")


def insert(s):
    """Write an insert, and a block when a value is a long binary string."""
    if "'\\x" not in s:
        return literal_dates(s)
    items, vi, vj, vk = values(s)
    head = s[:s.index("(")]
    cols = [c.strip() for c in s[s.index("(") + 1:s.upper().index(")", s.index("(")) ].split(",")]
    # The column list is the first parenthesis, and the values follow VALUES.
    cols = [c.strip() for c in re.search(r"\(([^)]*)\)\s*VALUES", s, re.I).group(1).split(",")]
    chunks, blobcol = [], None
    new = []
    for col, item in zip(cols, items):
        item = item.strip()
        if item.startswith("'\\x"):
            blobcol = col
            hexs = re.fullmatch(r"'\\x([0-9A-Fa-f]*)'(?:::bytea)?", item).group(1)
            chunks = [hexs[p:p + 4000] for p in range(0, len(hexs), 4000)]
            new.append("empty_blob()")
        else:
            new.append(literal_dates(item))
    table_name = re.match(r"insert into (\w+)", s, re.I).group(1)
    body = [
        "declare",
        "  b blob;",
        "begin",
        f"  insert into {table_name} ({', '.join(cols)})",
        f"  values ({', '.join(new)})",
        f"  returning {blobcol} into b;",
    ]
    for c in chunks:
        body.append(f"  dbms_lob.writeappend(b, {len(c) // 2}, hextoraw('{c}'));")
    body.append("end;")
    return "\n".join(body)


def view(s):
    s = strip_comments(s)
    # A price times a quantity times a discount is money in the PostgreSQL file.
    # In SQL Server the discount is a float, so the product is a float that
    # rounds to cents. Oracle multiplies a number and a BINARY_FLOAT in single
    # precision, so the subtraction stays in single precision, as in SQL Server,
    # and the product is a double that becomes a number.
    s = s.replace(
        "order_details.unit_price*quantity*((1-discount)::real)",
        "round(cast(order_details.unit_price*quantity*to_binary_double(1-discount) as number), 2)",
    )
    s = s.replace("2500::money", "2500")
    s = re.sub(r"'(\d{4})(\d{2})(\d{2})'", r"date '\1-\2-\3'", s)
    s = re.sub(r"avg\(unit_price::numeric\)::money", "avg(unit_price)", s)
    # Oracle takes no AS before the alias of a table.
    s = re.sub(r"\b(from|join)(\s+)(\w+) as (\w+)", r"\1\2\3 \4", s)
    return s


def unslash(src):
    """Turn statements that end with a slash line into statements that end with
    a semicolon. A block already ends with one."""
    out, buf = [], []

    def flush():
        text = "\n".join(buf).strip("\n")
        buf.clear()
        if not any(l.strip() and not l.strip().startswith("--") for l in text.split("\n")):
            if text.strip():
                out.append(text)
            return
        if not text.rstrip().endswith(";"):
            text = text.rstrip() + ";"
        out.append(text)

    for l in src.split("\n"):
        if l.strip() == "/":
            flush()
        else:
            buf.append(l)
    flush()
    return "\n".join(out) + "\n"


def main():
    src, dst, tail = sys.argv[1:4]
    statements = split(open(src).read())
    out = []
    indexed = set()
    for st in statements:
        st = re.sub(r"\A(?:\s*--[^\n]*\n)+", "", st).strip()
        low = st.lower()
        if low.startswith("create table"):
            name = re.match(r"create table (\w+)", low).group(1)
            for cols in re.findall(r"primary key \(([^)]*)\)", low):
                indexed.add((name, re.sub(r"\s", "", cols)))
            out.append(table(st))
        elif low.startswith("alter table") and "primary key" in low:
            name = re.match(r"alter table (\w+)", low).group(1)
            for cols in re.findall(r"primary key \(([^)]*)\)", low):
                indexed.add((name, re.sub(r"\s", "", cols)))
            out.append(table(st))
        elif low.startswith("create index"):
            m = re.match(r"create index \w+ on (\w+) \(([^)]*)\)", low)
            key = (m.group(1), re.sub(r"\s", "", m.group(2)))
            # Oracle refuses a second index on the same list of columns.
            if key in indexed:
                continue
            indexed.add(key)
            out.append(st)
        elif low.startswith("insert into"):
            out.append(insert(st))
        elif low.startswith("alter table"):
            out.append(table(st))
        elif low.startswith("create view"):
            out.append(view(st))
        elif low.startswith("select setval"):
            m = re.search(r"'(\w+)', '(\w+)'", st)
            out.append(f"alter table {m.group(1)} modify {m.group(2)} generated by default as identity (start with limit value)")
        elif low.startswith("create function") or low.startswith("--") or not low:
            continue
        else:
            raise SystemExit("an unknown statement: " + st[:60])
    text = "-- Northwind for Oracle, converted from the PostgreSQL script.\n"
    text += "\n/\n".join(out) + "\n/\n"
    text += open(tail).read().rstrip("\n") + "\n"
    open(dst, "w").write(unslash(text))


main()
