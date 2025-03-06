@echo on
REM Add logic to call a specific function based on the argument
if "%1"=="Setup-EventWriter" goto Setup-EventWriter
if "%1"=="Start-EventWriter" goto Start-EventWriter
if "%1"=="GetRetinaPromMetrics" goto GetRetinaPromMetrics
if "%1"=="CurlAkaMs" goto CurlAkaMs

goto :EOF

REM Define the Setup-EventWriter function
:Setup-EventWriter
   powershell -Command "Write-Output 'Listing contents of C:\'"
   dir C:\

   powershell -Command "Write-Output 'Copying event_writer.exe to C:'"
   copy .\event_writer.exe C:\event_writer.exe

    powershell -Command "Write-Output 'Copying bpf_event_writer.sys to C:'"
   copy .\bpf_event_writer.sys C:\bpf_event_writer.sys

   powershell -Command "Write-Output 'Listing contents of C:'"
   dir C:\

   goto :EOF

REM Define the Start-EventWriter function
:Start-EventWriter
   echo Changing directory to C:\
   cd C:\
   echo Starting event_writer.exe with -event %3 -srcIP %5
   .\event_writer.exe -event %3 -srcIP %5
   echo Changing directory to C:\hpc
   cd C:\hpc

   goto :EOF

REM Define the GetPromMetrics function
:GetRetinaPromMetrics
   echo Fetching Prometheus metrics from http://localhost:10093/metrics
   powershell -Command "Invoke-WebRequest -Uri 'http://localhost:10093/metrics' -UseBasicParsing | ForEach-Object { $_.Content }"

   goto :EOF

REM Curl AKA.MS
:CurlAkaMs
   // Hardcoding IP addr for aka.ms - 23.213.38.151
   echo Curl AKA.MS or 23.213.38.151
   start /B cmd /c "for /L %%i in (1,1,1000) do (powershell -Command \"Invoke-WebRequest -Uri 'http://23.213.38.151' -UseBasicParsing\" & timeout /t 1 >nul)"
   goto :EOF