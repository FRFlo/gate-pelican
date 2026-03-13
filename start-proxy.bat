@echo off
setlocal
cd /d "%~dp0"

if not exist "config.yml" (
  echo [Gate] Missing config.yml
  pause
  exit /b 1
)

if not exist "gate.exe" (
  echo [Gate] Missing gate.exe
  pause
  exit /b 1
)

echo [Gate] Starting proxy on 0.0.0.0:25565...
gate.exe --config "config.yml"

echo.
echo [Gate] Proxy stopped.
pause
