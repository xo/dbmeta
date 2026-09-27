# D104. The Couchbase model reads 7.6 and later

Status: Amends D94, D95 and D96.

`models/couchbase` is written, on the dbimp driver that D101 swapped in. It
answers 12 of the 55, and `docs/COVERAGE.md` holds what it answers and why.

## The floor is 7.6

D95 left the model three choices for 7.2, which sends the fields of a result
in name order. Ken chose on 2026-09-27 to raise the floor to 7.6. Every Scan
reads a column by its position, and 7.6 keeps the order the statement
selects. The other two choices were to name every column so that name order
and field order agree, or to read 7.2 by name. The first bends every query to
one old release, and the second is a second way of scanning for one product.

So every fragment gates on 7.6, and a 7.2 server reports `ErrVersionTooOld`
for every query. `TestCouchbaseTooOldBelowTheFloor` holds that on 7.2.9.
Measured on 7.2.9, the order is not the only difference: the signature of the
result is in name order too. Sequences also arrived in 7.6.

## The tiers

D94 made 7.2.9 and 8.0.3 Tested and 7.6.12 Nightly. That put the floor of the
model in the nightly run alone, so CI on every push checked the model only on
8.0.3. 7.6.12 is now Tested as well. 7.2.9 stays Tested, because the dbimp
driver supports it, and the push run proves that the model refuses it.

## What differs between 7.6 and 8.0

8.0 reserves the word `roles`, which 7.6 does not, so `u.roles` is a syntax
error there. The model quotes it, and it quoted `role` already. The refusal
message a lesser principal gets from `system:user_info` names a different
role on 8.0, so parity has a section `couchbase@8`.

## The one lesser principal

D96 expected parity to want an owner as well as a grantee, made with
`bucket_admin` on the bucket. Couchbase has no containment and no object
owner. A collection, an index and a function belong to a scope and not to a
user, so `bucket_admin` is more roles rather than another kind of principal.
Rule 16 asks for every lesser kind, and Couchbase has one, which is
`dbmeta_user` from D96. SQL++ cannot create a user, so the parity target
connects as that user and makes nobody.

The user is refused `Roles`, `RoleGrants` and `Privileges`, and sees no
function. `system:functions` shows only the functions a user can run or
manage, and the user holds no function role.

## The version statement

`usql` runs `SELECT RAW ds_version()` and prints "Couchbase" before it, and
dbmeta runs the same statement and names the product the same way.
`docs/USQL.md` has the row.

## The decision index regular expression

The test that checks references to decisions matched two digits alone, so a
reference to D100 or later went unchecked. It now matches any number, and
this is the first reference it caught before its decision existed.
