@echo off
setlocal EnableDelayedExpansion
set "SCRIPT_DIR=%~dp0"

if /I "%GDC_USE_PREBUILT%"=="1" (
  if exist "%SCRIPT_DIR%gdc.exe" (
    "%SCRIPT_DIR%gdc.exe" %*
    exit /b !ERRORLEVEL!
  )
  if exist "%SCRIPT_DIR%gdc-test.exe" (
    "%SCRIPT_DIR%gdc-test.exe" %*
    exit /b !ERRORLEVEL!
  )
  echo GDC_USE_PREBUILT=1 but no prebuilt Windows executable was found. 1>&2
  exit /b 1
)

go run "%SCRIPT_DIR%cmd\gdc" %*
