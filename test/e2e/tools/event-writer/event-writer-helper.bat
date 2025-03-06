@echo on
REM Add logic to call a specific function based on the argument
if "%1"=="Setup-EventWriter" goto Setup-EventWriter
if "%1"=="Start-EventWriter" goto Start-EventWriter
if "%1"=="GetRetinaPromMetrics" goto GetRetinaPromMetrics
if "%1"=="CurlAkaMs" goto CurlAkaMs

goto :EOF

REM Define the Setup-EventWriter function
:Setup-EventWriter
   copy .\event_writer.exe C:\event_writer.exe
   copy .\bpf_event_writer.sys C:\bpf_event_writer.sys

   goto :EOF

REM Define the Start-EventWriter function
:Start-EventWriter
   cd C:\
   start "" cmd .\event_writer.exe -event %3 -srcIP %5

   goto :EOF

REM Define the GetPromMetrics function
:GetRetinaPromMetrics
   powershell -Command "Invoke-WebRequest -Uri 'http://localhost:10093/metrics' -UseBasicParsing | ForEach-Object { $_.Content }"

   goto :EOF

REM Curl
:Curl
   powershell -Command "Write-Output 'Curl http://%2'"
   start "" cmd /c "for /L %%i in (1,1,1000) do (powershell -Command \"wget -Uri 'http://%2' -UseBasicParsing\" & timeout /t 1 >nul)"
   goto :EOF
