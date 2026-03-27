@echo off
setlocal

:: Vypnuti stare instance. Windows ma radsi zpetna lomitka.
if exist xrp.exe (
    .\xrp stop
) else (
    echo [INFO] xrp.exe nenalezeno, neni co zastavovat.
)

:: 1. Zkusime globalni PATH
where go >nul 2>nul
if %ERRORLEVEL% equ 0 (
    echo [INFO] Pouzivam globalni Go
    set "GO_CMD=go"
    goto :build
)

:: 2. Zkusime tvuj lokalni adresar (dynamicky, bez hardkodovani!)
if exist "%USERPROFILE%\.go\bin\go.exe" (
    echo [INFO] Pouzivam lokalni Go v %USERPROFILE%
    set "GO_CMD=%USERPROFILE%\.go\bin\go.exe"
    goto :build
)

:: 3. Zalozni standardni instalace ve Windows
if exist "C:\Program Files\Go\bin\go.exe" (
    echo [INFO] Pouzivam Program Files Go
    set "GO_CMD=C:\Program Files\Go\bin\go.exe"
    goto :build
)

:: Kdyz to nenajde vubec nikde
echo [ERROR] go.exe se nenaslo. Mas to vubec nainstalovany, nebo jsi rozbil PATH?
exit /b 1

:build
"%GO_CMD%" build ./cmd/xrp
if %ERRORLEVEL% equ 0 (
    echo [OK] Build uspesny.
) else (
    echo [ERROR] Build spadnul.
    exit /b %ERRORLEVEL%
)