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

# Install Google Cloud SDK (includes gcloud storage)
RUN curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee /etc/apt/sources.list.d/google-cloud-sdk.list \
    && apt-get update \
    && apt-get install -y google-cloud-cli \
    && rm -rf /var/lib/apt/lists/*

# Install Gemini CLI (pinned to known-working version)
RUN npm install -g @google/gemini-cli@0.56.0

# Set up workspace
RUN mkdir -p /workspace

WORKDIR /workspace
