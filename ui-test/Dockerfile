FROM docker.io/library/node:22.5.1@sha256:86915971d2ce1548842315fcce7cda0da59319a4dab6b9fc0827e762ef04683a as node

RUN apt-get update && apt-get install --no-install-recommends -y \
    software-properties-common 

#Chrome browser to run the tests
ARG CHROME_VERSION=86.0.4240.75
RUN curl https://dl-ssl.google.com/linux/linux_signing_key.pub | apt-key add \
    && wget https://www.slimjet.com/chrome/download-chrome.php?file=files%2F$CHROME_VERSION%2Fgoogle-chrome-stable_current_amd64.deb \
    && dpkg -i download-chrome*.deb || true

RUN apt-get install --no-install-recommends -y -f \
    && rm -rf /var/lib/apt/lists/*

#Disable the SUID sandbox so that chrome can launch without being in a privileged container
RUN dpkg-divert --add --rename --divert /opt/google/chrome/google-chrome.real /opt/google/chrome/google-chrome \
    && echo "#! /bin/bash\nexec /opt/google/chrome/google-chrome.real --no-sandbox --disable-setuid-sandbox \"\$@\"" > /opt/google/chrome/google-chrome \
     && chmod 755 /opt/google/chrome/google-chrome

WORKDIR /usr/src/app
COPY package*.json ./
COPY yarn.lock ./
RUN yarn install && yarn cache clean
COPY . .

