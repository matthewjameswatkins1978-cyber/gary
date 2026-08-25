FROM python:3.12-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    curl \
    wget \
    git \
    ca-certificates \
    gnupg \
    ripgrep \
    fd-find \
    jq \
    file \
    tree \
    less \
    procps \
    zip \
    unzip \
    xz-utils \
    build-essential \
    make \
    && rm -rf /var/lib/apt/lists/*

# fd-find installs as fdfind on Debian; symlink to fd
RUN ln -sf /usr/bin/fdfind /usr/local/bin/fd

# Install Go 1.24
RUN curl -fsSL https://go.dev/dl/go1.24.0.linux-amd64.tar.gz | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Install Node.js 20
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Google Cloud SDK via standalone installer (gcloud storage primary)
RUN curl -fsSL https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-linux-x86_64.tar.gz | tar -C /opt -xz \
    && /opt/google-cloud-sdk/install.sh --quiet --path-update true --rc-path /etc/bash.bashrc
ENV PATH="/opt/google-cloud-sdk/bin:${PATH}"
ENV CLOUDSDK_PYTHON="/usr/local/bin/python"

# Install gsutil via pip (gsutil fallback, uses Python 3.12)
RUN pip install --no-cache-dir gsutil

# Install Gemini CLI (pinned to known-working version)
RUN npm install -g @google/gemini-cli@0.56.0

# Set up workspace
RUN mkdir -p /workspace

WORKDIR /workspace
