FROM mcr.microsoft.com/dotnet/aspnet:11.0-alpine@sha256:9928cd451fef4fb35319eac3a5d7e48da7ce3f87c1018baac1a68ea5d5f8491c
RUN apk add bash
WORKDIR /app
ARG PROJECT_NAME
ENV PROJECT_NAME=$PROJECT_NAME
COPY ./app .
RUN chown -R $APP_UID:$APP_UID .
HEALTHCHECK --interval=1s --timeout=500ms --start-period=500ms --retries=30 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1/healthz || exit 1
USER $APP_UID
ENTRYPOINT dotnet "${PROJECT_NAME}.dll"
