FROM --platform=$BUILDPLATFORM node:20-alpine AS ui-builder
ARG BUILDTIME
WORKDIR /ui-app

# Copy the Web UI source (merged monorepo: docker/web-ui/)
COPY web-ui/ /ui-app/

# Install dependencies and build
RUN npm ci --no-audit --no-fund
RUN npm run build

FROM python:3.12.5-bookworm AS build
ARG no_mqttmanager_build
ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG IS_DEVEL
WORKDIR /MQTTManager
SHELL ["/bin/bash", "-c"]

COPY build_scripts/ /build_scripts/
COPY MQTTManager/ /MQTTManager/

# Only build MQTTManager inside Docker if no pre-built binary is provided
# (CI provides a pre-built binary via mqttmanager-build/ for releases)
RUN if [ -z "$no_mqttmanager_build" ]; then \
      apt-get update && apt-get -y upgrade && \
      echo "Running on $BUILDPLATFORM, building for $TARGETPLATFORM" && \
      bash /build_scripts/install_build_tools.sh "${IS_DEVEL}" && \
      /MQTTManager/compile_mqttmanager.sh --target-platform "$TARGETPLATFORM" --strip; \
    else \
      echo "Skipping MQTTManager build (pre-built binary provided)" && \
      mkdir -p /MQTTManager/build; \
    fi

FROM python:3.12.5-bookworm
ARG IS_DEVEL
WORKDIR /usr/src/app
SHELL ["/bin/bash", "-c"]

COPY build_scripts/ /build_scripts/
COPY web/requirements.txt /usr/src/app/requirements.txt
COPY nginx/sites-enabled/ /etc/nginx/sites-enabled/
COPY nginx/sites-templates/ /etc/nginx/sites-templates/

# Copy MQTTManager binary: from build stage (local dev) or pre-built (CI)
COPY --from=build /MQTTManager/build /MQTTManager/build/
COPY mqttmanager-build/ /MQTTManager/build/

# Update container
RUN apt-get update && apt-get -y upgrade

# Install software needed to build the manager (only in dev mode)
RUN if [ "$IS_DEVEL" == "yes" ]; then bash /build_scripts/install_build_tools.sh "${IS_DEVEL}"; fi

# Install ImageMagick to be able to process images and send them to the NSPanel
RUN apt-get install -y nginx libmagick++-dev
RUN pip install --no-cache-dir -r requirements.txt

# Everything that changes between releases
COPY "HMI_files/" /usr/src/app/nspanelmanager/HMI_files/
COPY web/ /usr/src/app/

EXPOSE 8000
COPY --from=ui-builder /ui-app/dist /var/www/html/webapp
CMD ["/bin/bash", "./run_uwsgi.sh"]
