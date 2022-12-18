#!/bin/bash

for entry in /in/*
do
    ffmpeg -i "$entry" -an -sn -c:0 libx264 -x264opts 'keyint=24:min-keyint=24:no-scenecut' -b:v 5300k -maxrate 5300k -bufsize 2650k -vf 'scale=-1:1080' /tmp/video-1080.mp4
    mp4fragment /tmp/video-1080.mp4 /tmp/f-video-1080.mp4
    mp4dash --mpd-name=manifest.mpd --output-dir=/out --force /tmp/f-video-1080.mp4
done

rm -rf /tmp/*