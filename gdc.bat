@echo off
setlocal
set "SCRIPT_DIR=%~dp0"

if exist "%SCRIPT_DIR%gdc.exe" (
  "%SCRIPT_DIR%gdc.exe" %*
  exit /b %ERRORLEVEL%
)

if exist "%SCRIPT_DIR%gdc-test.exe" (
  "%SCRIPT_DIR%gdc-test.exe" %*
  exit /b %ERRORLEVEL%
)

go run "%SCRIPT_DIR%cmd\gdc" %*
