#!/bin/bash
VER="v0.0.1"
#docker build --platform linux/arm64 -t xhub:$VER .
#docker save -o xhub:${VER}.linux-arm64.tar xhub:${VER}
docker build -t xhub:$VER .
