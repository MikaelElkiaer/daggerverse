FROM mcr.microsoft.com/dotnet/aspnet:9.0-alpine@sha256:4c3901bfc7d61d3e20e68f2034688dc83ac0c79d6c2b0e28eddd4b4ac0b9f5f4
RUN apk add bash
WORKDIR /app
ARG PROJECT_NAME
ENV PROJECT_NAME=$PROJECT_NAME
COPY ./app .
RUN chown -R $APP_UID:$APP_UID .
HEALTHCHECK --interval=1s --timeout=500ms --start-period=500ms --retries=30 CMD wget --no-verbose --tries=1 --spider http://127.0.0.1/healthz || exit 1
USER $APP_UID
ENTRYPOINT dotnet "${PROJECT_NAME}.dll"
