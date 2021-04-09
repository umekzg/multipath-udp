# See here for image contents: https://github.com/microsoft/vscode-dev-containers/tree/v0.166.1/containers/go/.devcontainer/base.Dockerfile

# [Choice] Go version: 1, 1.16, 1.15
ARG VARIANT="1.16"
FROM mcr.microsoft.com/vscode/devcontainers/go:0-${VARIANT}

# [Option] Install Node.js
ARG INSTALL_NODE="true"
ARG NODE_VERSION="lts/*"
RUN if [ "${INSTALL_NODE}" = "true" ]; then su vscode -c "umask 0002 && . /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1"; fi

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends tcl pkg-config libssl-dev openssl make cmake g++ bzip2 nasm

RUN git clone https://github.com/Haivision/srt /tmp/build/srt

WORKDIR /tmp/build/srt
RUN ./configure && make -j$(getconf _NPROCESSORS_ONLN) && make install
WORKDIR /

WORKDIR /tmp/build/ffmpeg
RUN wget -O ffmpeg-snapshot.tar.bz2 https://ffmpeg.org/releases/ffmpeg-snapshot.tar.bz2 && \
    tar xjvf ffmpeg-snapshot.tar.bz2

WORKDIR /tmp/build/ffmpeg/ffmpeg
RUN ./configure --enable-libsrt && make -j$(getconf _NPROCESSORS_ONLN) && make install
WORKDIR /