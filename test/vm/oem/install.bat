@echo off
rem ---------------------------------------------------------------------------
rem Installs SQL Server without a person watching.
rem
rem dockurr/windows copies this whole folder to C:\OEM and runs this file at the
rem end of the unattended Windows install, as SYSTEM. provision.sh writes the
rem four placeholder values below before it starts the machine.
rem
rem It writes C:\OEM\ready.txt on success and C:\OEM\failed.txt on failure,
rem and copies the log to the host after every step rather than only at the
rem end. Publishing only at the end meant that an install still running and an
rem install wedged halfway looked identical from Linux, which is the thing the
rem log was added to stop.
rem
rem dockur copies /oem into the image rather than mounting it, so nothing
rem written to C:\OEM reaches the host. The log is therefore also copied to the
rem directory mounted at /shared, which Windows shows as a drive, so a failed
rem install can be read from Linux instead of through the console. Finding that
rem out the hard way cost an afternoon on 2008 R2.
rem ---------------------------------------------------------------------------

set OEM=%~dp0
set LOG=%SystemDrive%\OEM\provision.log
set REGKEY=@@REGISTRY_KEY@@
set SAPWD=@@SA_PASSWORD@@
set INSTALLER=@@INSTALLER_FILE@@
set LICENSE=@@LICENSE_FLAG@@

echo [oem] starting %DATE% %TIME% > "%LOG%"

rem .NET Framework 3.5 is a prerequisite for 2008 R2, 2012 and 2014, and is not
rem present on Server 2012 R2 or later. It is part of Server 2008 R2 already,
rem where this call simply reports that and carries on.
rem The /all switch is not recognised by the DISM that ships with Server 2008
rem R2, which fails with "Error: 87". That release has .NET 3.5 built in, so
rem the whole step is optional there. Try the modern form, then the old one,
rem and carry on either way.
echo [oem] enabling .NET Framework 3.5 >> "%LOG%"
dism /online /enable-feature /featurename:NetFx3 /all /norestart >> "%LOG%" 2>&1
if errorlevel 1 dism /online /enable-feature /featurename:NetFx3 /norestart >> "%LOG%" 2>&1
call :publish running

rem The Express package is self extracting. The 2016 bootstrapper is not, and
rem takes its own switches, so it is handled on its own.
if /i "%INSTALLER:~-8%"=="Expr.exe" goto bootstrap

echo [oem] extracting %INSTALLER% >> "%LOG%"
"%OEM%%INSTALLER%" /Q /X:"%SystemDrive%\sqlsetup" >> "%LOG%" 2>&1
if not exist "%SystemDrive%\sqlsetup\setup.exe" goto failed
call :publish running

echo [oem] running setup >> "%LOG%"
"%SystemDrive%\sqlsetup\setup.exe" /ConfigurationFile="%OEM%ConfigurationFile.ini" ^
  /SAPWD="%SAPWD%" %LICENSE% >> "%LOG%" 2>&1
call :publish running
if errorlevel 1 goto failed
goto configure

:bootstrap
rem The 2016 package downloads the media, then installs from it. Two steps, so
rem that a download failure is distinguishable from an install failure.
echo [oem] downloading the 2016 media >> "%LOG%"
"%OEM%%INSTALLER%" /ACTION=Download /MEDIAPATH="%SystemDrive%\sqlmedia" /MEDIATYPE=Core /QUIET >> "%LOG%" 2>&1
if not exist "%SystemDrive%\sqlmedia\SQLEXPR_x64_ENU.exe" goto failed
echo [oem] extracting the 2016 media >> "%LOG%"
"%SystemDrive%\sqlmedia\SQLEXPR_x64_ENU.exe" /Q /X:"%SystemDrive%\sqlsetup" >> "%LOG%" 2>&1
if not exist "%SystemDrive%\sqlsetup\setup.exe" goto failed
echo [oem] running setup >> "%LOG%"
"%SystemDrive%\sqlsetup\setup.exe" /ConfigurationFile="%OEM%ConfigurationFile.ini" ^
  /SAPWD="%SAPWD%" %LICENSE% >> "%LOG%" 2>&1
if errorlevel 1 goto failed

:configure
rem Express listens on a dynamic port with TCP disabled, whatever TCPENABLED
rem said in the configuration file. This is the part that actually makes the
rem server reachable, and leaving it out is the usual reason one is not.
echo [oem] fixing the listening port >> "%LOG%"
set TCP=HKLM\SOFTWARE\Microsoft\Microsoft SQL Server\%REGKEY%\MSSQLServer\SuperSocketNetLib\Tcp
reg add "%TCP%" /v Enabled /t REG_DWORD /d 1 /f >> "%LOG%" 2>&1
reg add "%TCP%\IPAll" /v TcpPort /t REG_SZ /d 1433 /f >> "%LOG%" 2>&1
reg add "%TCP%\IPAll" /v TcpDynamicPorts /t REG_SZ /d "" /f >> "%LOG%" 2>&1

call :publish running
echo [oem] opening the firewall >> "%LOG%"
netsh advfirewall firewall add rule name="SQL Server 1433" dir=in action=allow protocol=TCP localport=1433 >> "%LOG%" 2>&1

echo [oem] restarting the service >> "%LOG%"
net stop MSSQLSERVER >> "%LOG%" 2>&1
net start MSSQLSERVER >> "%LOG%" 2>&1
if errorlevel 1 goto failed

rem Windows Server evaluation runs 180 days and rearms several times, which is
rem Microsoft's own way to extend it. Nothing here activates anything.
echo [oem] evaluation period >> "%LOG%"
cscript //nologo %SystemRoot%\System32\slmgr.vbs /xpr >> "%LOG%" 2>&1

echo [oem] done %DATE% %TIME% >> "%LOG%"
echo ready > "%SystemDrive%\OEM\ready.txt"
call :publish ready
exit /b 0

:failed
echo [oem] FAILED %DATE% %TIME% >> "%LOG%"
echo failed > "%SystemDrive%\OEM\failed.txt"
call :publish failed
exit /b 1

rem publish copies the log and the outcome to the host.
rem
rem dockur exposes the directory mounted at /shared over Samba as
rem \\host.lan\Data, and maps it to Z: at logon. The UNC path is used rather
rem than the drive letter, because the mapping is per user and made at logon,
rem and this runs as SYSTEM at the end of setup on 2016 and newer, where Z:
rem does not exist. The drive letters are tried afterwards anyway, for the
rem releases where this runs at logon instead.
:publish
copy /y "%LOG%" "\\host.lan\Data\provision-%COMPUTERNAME%.log" >nul 2>&1
echo %~1 > "\\host.lan\Data\outcome-%COMPUTERNAME%.txt" 2>nul
for %%d in (Z Y X W V U T S R Q P O N M L K J I H G F E D) do (
  if exist "%%d:\.shared" (
    copy /y "%LOG%" "%%d:\provision-%COMPUTERNAME%.log" >nul 2>&1
    echo %~1 > "%%d:\outcome-%COMPUTERNAME%.txt"
  )
)
exit /b 0
