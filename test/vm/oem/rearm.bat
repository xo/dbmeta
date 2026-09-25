@echo off
rem Extends the Windows evaluation period when it is nearly over.
rem
rem A Windows Server evaluation edition runs 180 days and then stops booting
rem into anything useful. slmgr /rearm resets the clock, which is Microsoft's
rem own mechanism rather than a way around one, and it works a finite number
rem of times: three on most of these editions, and the count cannot be reset.
rem
rem So this does not rearm on every boot. It reads the grace period first and
rem rearms only when less than ten days are left, which spends one rearm every
rem 170 days rather than one per reboot. A machine that is started often would
rem otherwise burn the whole budget in a week.
rem
rem It is registered by install.bat as a scheduled task that runs at startup,
rem as SYSTEM, because slmgr needs administrator rights.
rem
rem A rearm takes effect after a restart. This one does not restart the
rem machine: run.sh starts a machine and waits for SQL Server, and rebooting
rem underneath it looks exactly like a failed boot. The rearm applies at the
rem next start, and there is more than a week of grace left to reach it.

set LOG=%SystemDrive%\OEM\rearm.log
echo [rearm] %DATE% %TIME% >> "%LOG%"

rem GracePeriodRemaining is in minutes, and is 0 once the period is over.
rem wmic is present on every release here, which are Server 2008 R2 to 2016.
set GRACE=
for /f "tokens=2 delims==" %%a in ('wmic path SoftwareLicensingProduct where "PartialProductKey is not null" get GracePeriodRemaining /value 2^>nul ^| find "="') do set GRACE=%%a

if not defined GRACE (
  echo [rearm] could not read the grace period, doing nothing >> "%LOG%"
  exit /b 0
)
echo [rearm] %GRACE% minutes left >> "%LOG%"

rem Ten days in minutes.
if %GRACE% GEQ 14400 (
  echo [rearm] more than ten days left, doing nothing >> "%LOG%"
  exit /b 0
)

echo [rearm] rearming >> "%LOG%"
cscript //nologo %SystemRoot%\System32\slmgr.vbs /rearm >> "%LOG%" 2>&1
if errorlevel 1 (
  rem Out of rearms, or refused. Say so and leave the machine alone: this is
  rem the point at which a person has to rebuild it or let the release drop to
  rem Archived under D40.
  echo [rearm] FAILED, the rearm count is probably spent >> "%LOG%"
  exit /b 1
)
echo [rearm] done, it applies at the next start >> "%LOG%"
exit /b 0
