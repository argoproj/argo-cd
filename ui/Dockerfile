FROM node:11.15.0 as build

WORKDIR /src
ADD ["package.json", "yarn.lock", "./"]

RUN yarn install

ADD [".", "."]

ARG ARGO_VERSION=latest
ENV ARGO_VERSION=$ARGO_VERSION
RUN NODE_ENV='production' yarn build

FROM alpine:3.7

COPY  --from=build ./src/dist/app /app
