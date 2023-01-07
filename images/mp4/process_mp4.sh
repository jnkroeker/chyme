#!/bin/bash

for entry in /in/*
do
    mp4fragment "$entry" /tmp/f-"$entry"
    mp4dash --mpd-name=manifest.mpd --output-dir=/out --force /tmp/f-"$entry"
    # start metadata extraction
    ffmpeg -y -i "$entry" -codec copy -map 0:3 -f rawvideo /tmp/"$entry".bin
    cd /home/john
    ./gopro -i /tmp/"$entry".bin -o /out/"$entry".json
    # end metadata extraction
done

rm -rf /tmp/*