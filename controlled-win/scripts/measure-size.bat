@echo off
:: Windows 资源优化测量脚本（PowerShell / cmd 都可跑）
:: 用法：scripts\measure-size.bat
setlocal
cd /d "%~dp0\.."

if "%~1"=="" (set TAG=dev) else (set TAG=%1)
set OUT=target\measure
if not exist "%OUT%" mkdir "%OUT%"

echo ==^> building release...
cargo build --release --target-dir target
if errorlevel 1 exit /b 1

for /f "delims=" %%i in ('dir /b /a-d target\release\linkall-controlled-win* 2^>nul') do set BIN=target\release\%%i
if "%BIN%"=="" (
  echo no binary found in target\release\
  exit /b 1
)

echo ==^> size...
dir "%BIN%" > "%OUT%\size-%TAG%.txt"
powershell -NoProfile -Command "Get-Item '%BIN%' | Format-List Length, LastWriteTime" >> "%OUT%\size-%TAG%.txt"

where cargo-bloat >nul 2>&1
if %errorlevel% equ 0 (
  echo ==^> cargo-bloat analysis...
  cargo bloat --release --target-dir target -n 30 > "%OUT%\bloat-%TAG%.txt" 2>nul
  cargo bloat --release --target-dir target --crates -n 30 > "%OUT%\bloat-crates-%TAG%.txt" 2>nul
) else (
  echo cargo-bloat not installed. Install with: cargo install cargo-bloat
)

where dumpbin >nul 2>&1
if %errorlevel% equ 0 (
  echo ==^> DLL dependencies...
  dumpbin /dependents "%BIN%" > "%OUT%\dlls-%TAG%.txt"
)

echo Done. Results in %OUT%\
dir /b "%OUT%\"
endlocal
