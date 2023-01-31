#!/bin/bash

for entry in /in/*
do
    ffmpeg -y -i "$entry" -codec copy -map 0:3 -f rawvideo /tmp/video-1080.bin
    cd /home/john
    ./gopro -i /tmp/video-1080.bin -o /out/metadata.json
    mp4fragment "$entry" /tmp/f-video-1080.mp4
    mp4dash --mpd-name=manifest.mpd --output-dir=/out --force /tmp/f-video-1080.mp4
done

rm -rf /tmp/*