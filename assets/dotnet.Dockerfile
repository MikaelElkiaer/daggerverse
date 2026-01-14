FROM mcr.microsoft.com/dotnet/aspnet:8.0-alpine@sha256:a0ce42fe86548363a9602c47fc3bd4cf9c35a2705c68cd98d7ce18ae8735b83c
RUN apk add bash
WORKDIR /app
ARG PROJECT_NAME
ENV PROJECT_NAME=$PROJECT_NAME
COPY ./app .
RUN chown -R $APP_UID:$APP_UID .
HEALTHCHECK --interval=1s --timeout=500ms --start-period=500ms --retries=30 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1/healthz || exit 1
USER $APP_UID
ENTRYPOINT dotnet "${PROJECT_NAME}.dll"
