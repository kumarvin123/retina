@echo on
REM Add logic to call a specific function based on the argument
if "%1"=="Setup-EventWriter" goto Setup-EventWriter
if "%1"=="Start-EventWriter" goto Start-EventWriter
if "%1"=="GetRetinaPromMetrics" goto GetRetinaPromMetrics
if "%1"=="CurlExampleCOM" goto CurlExampleCOM
if "%1"=="DumpEventWriter" goto DumpEventWriter
if "%1"=="DumpCurl" goto DumpCurl
if "%1"=="PinMaps" goto Pin-Maps
goto :EOF

REM Define the Setup-EventWriter function
:Setup-EventWriter
   copy .\event_writer.exe C:\event_writer.exe
   copy .\bpf_event_writer.sys C:\bpf_event_writer.sys
   goto :EOF

REM Define the Start-EventWriter function .\event_writer.exe -event %3 -srcIP %5
:Start-EventWriter
   cd C:\
   start /B cmd /c ".\event_writer.exe -event %3 -srcIP %5 > C:\event_writer.out 2>&1"
   goto :EOF

REM Define the Pin-Maps
:PinMaps
   cd C:\
   start /B cmd /c ".\event_writer.exe -pinmaps > C:\event_writer.out 2>&1"
   goto :EOF

REM Define the GetPromMetrics function
:GetRetinaPromMetrics
   curl http://localhost:10093/metrics
   goto :EOF

REM Curl
:CurlExampleCOM
   start /B cmd /c "for /L %i in (1,1,10) do (curl ""http://23.192.228.84"" >> C:\curl.out 2>&1 & timeout /t 1 >nul)"
   goto :EOF

REM Dump Event Writer output
:DumpEventWriter
   type C:\event_writer.out
   goto :EOF

REM Dump Curl Output
:DumpCurl
   type C:\curl.out
   goto :EOF