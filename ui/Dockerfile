FROM node:11.15.0 as build

WORKDIR /src
ADD ["package.json", "yarn.lock", "./"]

RUN yarn install

ADD [".", "."]

ARG ARGO_VERSION=latest
ENV ARGO_VERSION=$ARGO_VERSION
RUN NODE_ENV='production' yarn build && yarn cache clean && yarn install --production

FROM node:11.15.0-alpine

COPY  --from=build ./src/dist /app
COPY  --from=build ./src/node_modules /app/node_modules
WORKDIR /app

EXPOSE 8001
CMD node api/api/main.js --uiDist /app/app --inCluster ${IN_CLUSTER} --namespace ${ARGO_NAMESPACE} --force-namespace-isolation ${FORCE_NAMESPACE_ISOLATION} --instanceId ${INSTANCE_ID:-''} --enableWebConsole ${ENABLE_WEB_CONSOLE:-'false'} --uiBaseHref ${BASE_HREF:-'/'} --ip ${IP:-'0.0.0.0'} --port ${PORT:-'8001'}
