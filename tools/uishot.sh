#!/bin/bash
# usage: shot.sh name page size [theme] [lang]
F=$(ls /c/Users/dmitriy.lyakhov/AppData/Local/Microsoft/WinGet/Packages/Gyan.FFmpeg*/ffmpeg-*/bin/ffmpeg.exe | head -1)
OUT=/c/Users/dmitriy.lyakhov/Projects/Veil/dist/shots; mkdir -p $OUT
VEIL_MULTI=1 VEIL_DEMO=$2 VEIL_SIZE=$3 VEIL_THEME=$4 VEIL_LANG=$5 /tmp/veil-demo.exe >/dev/null 2>&1 &
sleep 5
"$F" -y -loglevel error -f gdigrab -i title=Veil -frames:v 1 $OUT/$1.png
taskkill //IM veil-demo.exe //F >/dev/null 2>&1
