FROM ghcr.io/astral-sh/uv:latest
ARG NEXTDNS_ID
ARG NEXTDNS_EXT_ID
WORKDIR /app

COPY pyproject.toml main.py .

ENV NEXTDNS_ENDPOINT="https://link-ip.nextdns.io/${NEXTDNS_ID}/${NEXTDNS_EXT_ID}"
ENV UPDATE_INTERVAL_SECONDS=300

CMD ["uv", "run", "main.py"]
