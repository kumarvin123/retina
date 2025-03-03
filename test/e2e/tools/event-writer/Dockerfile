FROM mcr.microsoft.com/windows/servercore:ltsc2022 As base
WORKDIR C:\
COPY ./install-ebpf-xdp.ps1 ./install-ebpf-xdp.ps1
COPY ./x64/Release/bpf_event_writer.sys ./bpf_event_writer.sys
COPY ./x64/Release/event_writer.exe ./event_writer.exe