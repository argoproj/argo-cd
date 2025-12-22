FROM docker.io/library/node:23.0.0@sha256:9d09fa506f5b8465c5221cbd6f980e29ae0ce9a3119e2b9bc0842e6a3f37bb59 as node

RUN apt-get update && apt-get install --no-install-recommends -y \
    software-properties-common 

#Chrome browser to run the tests
ARG CHROME_VERSION=144.0.7559.27
RUN wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb \
    && dpkg -i google-chrome*.deb || true

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

ENTRYPOINT ["bash", "-c"]
CMD ["npm rum compile && npm rum pretest && npm rum test"]
