@echo on
REM Add logic to call a specific function based on the argument
if "%1"=="Setup-EventWriter" goto Setup-EventWriter
if "%1"=="Start-EventWriter" goto Start-EventWriter
if "%1"=="GetRetinaPromMetrics" goto GetRetinaPromMetrics

goto :EOF

REM Define the Setup-EventWriter function
:Setup-EventWriter
   echo Listing contents of C:\
   dir C:\

   echo Copying event_writer.exe to C:\
   copy .\event_writer.exe C:\event_writer.exe

   echo Copying bpf_event_writer.sys to C:\
   copy .\bpf_event_writer.sys C:\bpf_event_writer.sys

   echo Listing contents of C:\
   dir C:\
   goto :EOF

REM Define the Start-EventWriter function
:Start-EventWriter
   echo Changing directory to C:\
   cd C:\

   echo Starting event_writer.exe with -event 4
   start .\event_writer.exe -event 4

   echo Changing directory to C:\hpc
   cd C:\hpc

   goto :EOF

REM Define the GetPromMetrics function
:GetRetinaPromMetrics
   echo Fetching Prometheus metrics from http://localhost:10093/metrics
   powershell -Command "Invoke-WebRequest -Uri 'http://localhost:10093/metrics' -UseBasicParsing | ForEach-Object { $_.Content }"

   goto :EOF