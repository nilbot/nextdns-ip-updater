# NextDNS IP Updater

A simple Docker-based service that automatically updates NextDNS with your current WAN IP address at regular intervals by directly calling the NextDNS endpoint from within your home network.

## Features

- Regularly updates the NextDNS endpoint with your WAN IP
- NextDNS automatically detects your WAN IP when called from your network
- Uses structured logging for better visibility
- Runs in a Docker container for easy deployment
- Configurable update interval

## Configuration

Configuration is done through environment variables:

- `NEXTDNS_ENDPOINT`: The NextDNS endpoint URL to update (default: https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID)
- `UPDATE_INTERVAL_SECONDS`: Time between updates in seconds (default: 300 seconds / 5 minutes)

## Usage

### Using Docker Compose (recommended)

1. Clone this repository
2. (Optional) Edit the `docker-compose.yml` file to change the environment variables
3. Run:

```bash
docker-compose up -d
```

### Using Docker directly

```bash
docker build -t nextdns-ip-updater .
docker run -d --name nextdns-ip-updater \
  -e NEXTDNS_ENDPOINT=https://link-ip.nextdns.io/YOUR_ID/YOUR_EXT_ID \
  -e UPDATE_INTERVAL_SECONDS=300 \
  nextdns-ip-updater
```

## Logs

View the logs with:

```bash
docker logs -f nextdns-ip-updater
```

## Security Considerations

This image runs as the default (root) user. For production environments, consider implementing additional security measures.
