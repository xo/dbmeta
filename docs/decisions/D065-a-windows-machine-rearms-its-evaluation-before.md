# D65. A Windows machine rearms its evaluation before it expires

Status: Decided.

`test/cmd/dbrun/oem/rearm.bat` runs at every startup as a scheduled task, reads the
grace period, and spends a rearm only when fewer than ten days are left.

## Why not on every boot

The rearm count is finite, three on most of these editions, and it cannot be
reset. If it rearms on every boot, a machine that is started often spends the
whole budget in a week and is no better off. Reading `GracePeriodRemaining`
first turns that into one rearm every 170 days.

## Why it does not reboot

A rearm applies at the next start. `dbrun` starts a machine and waits for
SQL Server, and a reboot underneath that looks exactly like a machine that
failed to come up. There is more than a week of grace left when the rearm runs,
so the next ordinary start is soon enough.

## When the rearms are gone

`C:\OEM\rearm.log` says so and nothing else happens. At that point the machine
is rebuilt, which is about an hour, or the release drops to Archived under D40
and nothing is claimed for it. Neither is automatic, because both are a
person's decision about how much a pre-2017 SQL Server is worth.
