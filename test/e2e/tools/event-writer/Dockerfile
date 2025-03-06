#FROM mcr.microsoft.com/windows/servercore:ltsc2022 As base

#COPY ./install-ebpf-xdp.ps1 ./install-ebpf-xdp.ps1
#COPY ./x64/Release/bpf_event_writer.sys ./bpf_event_writer.sys
#COPY ./x64/Release/event_writer.exe ./event_writer.exe
#COPY ./event-writer-helper.bat ./event-writer-helper.bat

# Stage 1: Install ThreadJob
FROM mcr.microsoft.com/powershell:7.3-windowsservercore-ltsc2022
RUN pwsh -Command "Install-Module -Name ThreadJob -Force -Scope AllUsers -DestinationPath ."

COPY ./ThreadJob.psm1 ./ThreadJob.psm1
COPY ./install-ebpf-xdp.ps1 ./install-ebpf-xdp.ps1
COPY ./x64/Release/bpf_event_writer.sys ./bpf_event_writer.sys
COPY ./x64/Release/event_writer.exe ./event_writer.exe
COPY ./event-writer-helper.bat ./event-writer-helper.bat